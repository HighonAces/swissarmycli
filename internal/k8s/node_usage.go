package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"text/tabwriter"

	"github.com/HighonAces/swissarmycli/internal/k8s/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"k8s.io/metrics/pkg/client/clientset/versioned"
)

// ShowNodeUsage displays CPU and memory requests and limits for all nodes
func ShowNodeUsage() error {
	clientset, err := common.GetKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	metricsClient, err := common.GetMetricsClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create metrics client: %v. Usage data will be unavailable.\n", err)
	}

	fmt.Println("Fetching node resource usage information...")

	// Fetch all data concurrently
	var wg sync.WaitGroup
	var nodes *corev1.NodeList
	var pods *corev1.PodList
	var nodeMetrics *metricsv1beta1.NodeMetricsList
	var nodeErr, podErr, metricsErr error

	wg.Add(2)
	
	go func() {
		defer wg.Done()
		nodes, nodeErr = clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	}()
	
	go func() {
		defer wg.Done()
		pods, podErr = clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	}()

	if metricsClient != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nodeMetrics, metricsErr = metricsClient.MetricsV1beta1().NodeMetricses().List(context.TODO(), metav1.ListOptions{})
		}()
	}

	wg.Wait()

	if nodeErr != nil {
		return fmt.Errorf("failed to get nodes: %w", nodeErr)
	}
	if podErr != nil {
		return fmt.Errorf("failed to get pods: %w", podErr)
	}

	// Build node stats
	nodeStats := make(map[string]*nodeInfo)
	for _, node := range nodes.Items {
		nodeStats[node.Name] = &nodeInfo{
			name:           node.Name,
			cpuCapacity:    float64(node.Status.Capacity.Cpu().MilliValue()) / 1000,
			memoryCapacity: float64(node.Status.Capacity.Memory().Value()) / (1024 * 1024 * 1024),
		}
	}

	// Process pods
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning || pod.Spec.NodeName == "" {
			continue
		}

		nodeInfo := nodeStats[pod.Spec.NodeName]
		if nodeInfo == nil {
			continue
		}

		for _, container := range pod.Spec.Containers {
			if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
				nodeInfo.cpuRequests += float64(cpu.MilliValue()) / 1000
			}
			if memory, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
				nodeInfo.memoryRequests += float64(memory.Value()) / (1024 * 1024 * 1024)
			}
			if cpu, ok := container.Resources.Limits[corev1.ResourceCPU]; ok {
				nodeInfo.cpuLimits += float64(cpu.MilliValue()) / 1000
			}
			if memory, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
				nodeInfo.memoryLimits += float64(memory.Value()) / (1024 * 1024 * 1024)
			}
		}
	}

	// Add metrics data
	if nodeMetrics != nil && metricsErr == nil {
		for _, metric := range nodeMetrics.Items {
			if nodeInfo, exists := nodeStats[metric.Name]; exists {
				nodeInfo.cpuUsage = float64(metric.Usage.Cpu().MilliValue()) / 1000
				nodeInfo.memoryUsage = float64(metric.Usage.Memory().Value()) / (1024 * 1024 * 1024)
			}
		}
	}

	// Output results
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NODE\tCPU CAPACITY\tCPU REQUESTS\tCPU LIMITS\tCPU USAGE\tMEMORY CAPACITY\tMEMORY REQUESTS\tMEMORY LIMITS\tMEMORY USAGE")

	for _, nodeInfo := range nodeStats {
		cpuUsage := "N/A"
		memoryUsage := "N/A"
		if nodeInfo.cpuUsage > 0 {
			cpuUsage = fmt.Sprintf("%.2f (%.0f%%)", nodeInfo.cpuUsage, nodeInfo.cpuUsage*100/nodeInfo.cpuCapacity)
		}
		if nodeInfo.memoryUsage > 0 {
			memoryUsage = fmt.Sprintf("%.2fGi (%.0f%%)", nodeInfo.memoryUsage, nodeInfo.memoryUsage*100/nodeInfo.memoryCapacity)
		}

		fmt.Fprintf(w, "%s\t%.2f\t%.2f (%.0f%%)\t%.2f (%.0f%%)\t%s\t%.2fGi\t%.2fGi (%.0f%%)\t%.2fGi (%.0f%%)\t%s\n",
			nodeInfo.name,
			nodeInfo.cpuCapacity,
			nodeInfo.cpuRequests, nodeInfo.cpuRequests*100/nodeInfo.cpuCapacity,
			nodeInfo.cpuLimits, nodeInfo.cpuLimits*100/nodeInfo.cpuCapacity,
			cpuUsage,
			nodeInfo.memoryCapacity,
			nodeInfo.memoryRequests, nodeInfo.memoryRequests*100/nodeInfo.memoryCapacity,
			nodeInfo.memoryLimits, nodeInfo.memoryLimits*100/nodeInfo.memoryCapacity,
			memoryUsage)
	}

	w.Flush()
	return nil
}

type nodeInfo struct {
	name           string
	cpuCapacity    float64
	cpuRequests    float64
	cpuLimits      float64
	cpuUsage       float64
	memoryCapacity float64
	memoryRequests float64
	memoryLimits   float64
	memoryUsage    float64
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
