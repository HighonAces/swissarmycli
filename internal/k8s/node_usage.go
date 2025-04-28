package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/HighonAces/swissarmycli/internal/k8s/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
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
		memoryCapacity := node.Status.Capacity.Memory().Value() / (1024 * 1024) // Convert to MiB

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
		var memoryRequests, memoryLimits int64

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
					memoryRequests += memory.Value() / (1024 * 1024) // Convert to MiB
				}

				if cpu, ok := limits[corev1.ResourceCPU]; ok {
					cpuLimits += cpu.MilliValue()
				}
				if memory, ok := limits[corev1.ResourceMemory]; ok {
					memoryLimits += memory.Value() / (1024 * 1024) // Convert to MiB
				}
			}
		}

		// Display CPU usage in millicores and memory in MiB
		var cpuUsage, memoryUsage string
		if nodeMetrics != nil {
			cpuUsageValue := nodeMetrics.Usage.Cpu().MilliValue()
			memoryUsageValue := nodeMetrics.Usage.Memory().Value() / (1024 * 1024) // Convert to MiB
			cpuUsage = fmt.Sprintf("%dm (%d%%)", cpuUsageValue, cpuUsageValue*100/cpuCapacity)
			memoryUsage = fmt.Sprintf("%dMi (%d%%)", memoryUsageValue, memoryUsageValue*100/memoryCapacity)
		} else {
			cpuUsage = "N/A"
			memoryUsage = "N/A"
		}

		// Print row for this node
		fmt.Fprintf(w, "%s\t%dm\t%dm (%d%%)\t%dm (%d%%)\t%s\t%dMi\t%dMi (%d%%)\t%dMi (%d%%)\t%s\n",
			nodeName,
			cpuCapacity,
			cpuRequests, cpuRequests*100/cpuCapacity,
			cpuLimits, cpuLimits*100/cpuCapacity,
			cpuUsage,
			memoryCapacity,
			memoryRequests, memoryRequests*100/memoryCapacity,
			memoryLimits, memoryLimits*100/memoryCapacity,
			memoryUsage)
	}

	w.Flush()
	return nil
}

// getKubernetesClients creates the Kubernetes clientset and metrics clientset
func getKubernetesClients() (*kubernetes.Clientset, *versioned.Clientset, error) {
	// Find home directory for kubeconfig
	home := homedir.HomeDir()
	kubeconfigPath := filepath.Join(home, ".kube", "config")

	// Override with KUBECONFIG env var if present
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		kubeconfigPath = kubeconfig
	}

	// Build config from kubeconfig file
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error building kubeconfig: %w", err)
	}

	// Create clientset for API resources
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	// Create metrics clientset for node metrics
	metricsClient, err := versioned.NewForConfig(config)
	if err != nil {
		return clientset, nil, fmt.Errorf("error creating Metrics client: %w", err)
	}

	return clientset, metricsClient, nil
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
