package k8s

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/HighonAces/swissarmycli/internal/k8s/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"k8s.io/metrics/pkg/client/clientset/versioned"
)

// ShowNodeUsage displays CPU and memory requests and limits for all nodes
func ShowNodeUsage() error {
	clientset, err := common.GetKubernetesClient() // Use the new public function
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	metricsClient, err := common.GetMetricsClient() // Use the new public function
	if err != nil {
		// Decide how to handle metrics client failure. Maybe just log and continue?
		fmt.Fprintf(os.Stderr, "Warning: could not create metrics client: %v. Usage data will be unavailable.\n", err)
		// metricsClient will be nil, the rest of the code needs to handle this gracefully
	}

	// Get all nodes
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to get nodes: %w", err)
	}

	fmt.Println("Fetching node resource usage information...")
	const divisorGiB = float64(1024 * 1024 * 1024)
	// Create a new tabwriter to format the output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NODE\tCPU CAPACITY\tCPU REQUESTS\tCPU LIMITS\tCPU USAGE\tMEMORY CAPACITY\tMEMORY REQUESTS\tMEMORY LIMITS\tMEMORY USAGE")

	// Process each node
	for _, node := range nodes.Items {
		nodeName := node.Name

		// Get node metrics
		nodeMetrics, err := getNodeMetrics(metricsClient, nodeName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not get metrics for node %s: %v\n", nodeName, err)
		}

		// Get node capacity
		cpuCapacity := node.Status.Capacity.Cpu().MilliValue()
		memoryCapacity := float64(node.Status.Capacity.Memory().Value()) / divisorGiB // Convert to MiB

		// Get all pods on this node
		fieldSelector := fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName})
		pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
			FieldSelector: fieldSelector.String(),
		})
		if err != nil {
			return fmt.Errorf("failed to get pods for node %s: %w", nodeName, err)
		}

		// Calculate total resource requests and limits for the node
		var cpuRequests, cpuLimits int64
		var memoryRequests, memoryLimits float64

		for _, pod := range pods.Items {
			// Skip pods that are not running
			if pod.Status.Phase != corev1.PodRunning {
				continue
			}

			// Sum resource requests and limits from all containers in the pod
			for _, container := range pod.Spec.Containers {
				requests := container.Resources.Requests
				limits := container.Resources.Limits

				if cpu, ok := requests[corev1.ResourceCPU]; ok {
					cpuRequests += cpu.MilliValue()
				}
				if memory, ok := requests[corev1.ResourceMemory]; ok {
					memoryRequests += float64(memory.Value()) / divisorGiB // Convert to MiB
				}

				if cpu, ok := limits[corev1.ResourceCPU]; ok {
					cpuLimits += cpu.MilliValue()
				}
				if memory, ok := limits[corev1.ResourceMemory]; ok {
					memoryLimits += float64(memory.Value()) / divisorGiB // Convert to MiB
				}
			}
		}

		// Display CPU usage in millicores and memory in MiB
		var cpuUsage, memoryUsage string
		if nodeMetrics != nil {
			cpuUsageValue := nodeMetrics.Usage.Cpu().MilliValue()
			cpuUsagePercent := int64(0)
			if cpuCapacity > 0 { // Avoid division by zero
				cpuUsagePercent = cpuUsageValue * 100 / cpuCapacity
			}
			memoryUsageValue := float64(nodeMetrics.Usage.Memory().Value()) / divisorGiB // Convert to MiB
			memoryUsagePercent := float64(0.0)                                           // Default to 0.0
			if memoryCapacity > 0.0 {                                                    // Avoid division by zero (use float comparison)
				memoryUsagePercent = (memoryUsageValue * 100.0) / memoryCapacity
			}
			cpuUsage = fmt.Sprintf("%dm (%d%%)", cpuUsageValue, cpuUsagePercent)
			memoryUsage = fmt.Sprintf("%dMi (%d%%)", memoryUsageValue, memoryUsagePercent)
		} else {
			cpuUsage = "N/A"
			memoryUsage = "N/A"
		}

		// --- CHANGE: Calculate percentages using float for memory ---
		cpuReqPercent := int64(0)
		cpuLimPercent := int64(0)
		memReqPercent := float64(0.0)
		memLimPercent := float64(0.0)

		if cpuCapacity > 0 {
			cpuReqPercent = cpuRequests * 100 / cpuCapacity
			cpuLimPercent = cpuLimits * 100 / cpuCapacity
		}
		if memoryCapacity > 0.0 { // Use float comparison
			memReqPercent = (memoryRequests * 100.0) / memoryCapacity
			memLimPercent = (memoryLimits * 100.0) / memoryCapacity
		}

		// Print row for this node
		fmt.Fprintf(w, "%s\t%dm\t%dm (%d%%)\t%dm (%d%%)\t%s\t%.2fGiB\t%.2fGiB (%.1f%%)\t%.2fGiB (%.1f%%)\t%s\n",
			nodeName,
			cpuCapacity,
			cpuRequests, cpuReqPercent,
			cpuLimits, cpuLimPercent,
			cpuUsage,
			memoryCapacity,
			memoryRequests, memReqPercent,
			memoryLimits, memLimPercent,
			memoryUsage)
	}

	w.Flush()
	return nil
}

// getNodeMetrics fetches metrics for a specific node
func getNodeMetrics(metricsClient *versioned.Clientset, nodeName string) (*metricsv1beta1.NodeMetrics, error) {
	if metricsClient == nil {
		return nil, fmt.Errorf("metrics client not initialized")
	}

	metrics, err := metricsClient.MetricsV1beta1().NodeMetricses().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return metrics, nil
}
