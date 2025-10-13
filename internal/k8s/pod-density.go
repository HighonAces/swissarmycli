package k8s

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"text/tabwriter"

	"github.com/HighonAces/swissarmycli/internal/k8s/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

type OwnerInfo struct {
	Name       string
	Type       string
	Namespace  string
	PodCount   int
	CPURequest float64
	CPULimit   float64
	MemRequest float64
	MemLimit   float64
}

type NodeInfo struct {
	Name           string
	PodCount       int
	CPUCapacity    float64
	CPURequests    float64
	CPULimits      float64
	CPUUsage       float64
	MemoryCapacity float64
	MemoryRequests float64
	MemoryLimits   float64
	MemoryUsage    float64
	Owners         []*OwnerInfo
}

func ShowPodDensity() error {
	clientset, err := common.GetKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	metricsClient, err := common.GetMetricsClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create metrics client: %v. Usage data will be unavailable.\n", err)
	}

	var wg sync.WaitGroup
	var nodes *corev1.NodeList
	var pods *corev1.PodList
	var replicaSets *appsv1.ReplicaSetList
	var nodeMetrics *metricsv1beta1.NodeMetricsList
	var nodeErr, podErr, rsErr, metricsErr error

	// Fetch all data concurrently
	wg.Add(3)
	
	go func() {
		defer wg.Done()
		nodes, nodeErr = clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	}()
	
	go func() {
		defer wg.Done()
		pods, podErr = clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	}()
	
	go func() {
		defer wg.Done()
		replicaSets, rsErr = clientset.AppsV1().ReplicaSets("").List(context.TODO(), metav1.ListOptions{})
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
	if rsErr != nil {
		return fmt.Errorf("failed to get replicasets: %w", rsErr)
	}

	rsOwnerCache := make(map[string]string)
	for _, rs := range replicaSets.Items {
		for _, owner := range rs.OwnerReferences {
			if owner.Kind == "Deployment" {
				rsOwnerCache[rs.Namespace+"/"+rs.Name] = owner.Name
			}
		}
	}

	nodeMap := make(map[string]map[string]*OwnerInfo)
	nodeStats := make(map[string]*NodeInfo)

	for _, node := range nodes.Items {
		nodeStats[node.Name] = &NodeInfo{
			Name:           node.Name,
			CPUCapacity:    float64(node.Status.Capacity.Cpu().MilliValue()) / 1000,
			MemoryCapacity: float64(node.Status.Capacity.Memory().Value()) / (1024 * 1024 * 1024),
		}
		nodeMap[node.Name] = make(map[string]*OwnerInfo)
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning || pod.Spec.NodeName == "" {
			continue
		}

		nodeName := pod.Spec.NodeName
		owner, ownerType := getPodOwnerFast(&pod, rsOwnerCache)
		key := fmt.Sprintf("%s/%s/%s", pod.Namespace, ownerType, owner)

		if nodeMap[nodeName][key] == nil {
			nodeMap[nodeName][key] = &OwnerInfo{
				Name:      owner,
				Type:      ownerType,
				Namespace: pod.Namespace,
			}
		}

		ownerInfo := nodeMap[nodeName][key]
		ownerInfo.PodCount++

		for _, container := range pod.Spec.Containers {
			if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
				cpuCores := float64(cpu.MilliValue()) / 1000
				ownerInfo.CPURequest += cpuCores
				nodeStats[nodeName].CPURequests += cpuCores
			}
			if cpu, ok := container.Resources.Limits[corev1.ResourceCPU]; ok {
				cpuCores := float64(cpu.MilliValue()) / 1000
				ownerInfo.CPULimit += cpuCores
				nodeStats[nodeName].CPULimits += cpuCores
			}
			if mem, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
				memGi := float64(mem.Value()) / (1024 * 1024 * 1024)
				ownerInfo.MemRequest += memGi
				nodeStats[nodeName].MemoryRequests += memGi
			}
			if mem, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
				memGi := float64(mem.Value()) / (1024 * 1024 * 1024)
				ownerInfo.MemLimit += memGi
				nodeStats[nodeName].MemoryLimits += memGi
			}
		}
	}

	if nodeMetrics != nil && metricsErr == nil {
		for _, metric := range nodeMetrics.Items {
			if nodeInfo, exists := nodeStats[metric.Name]; exists {
				nodeInfo.CPUUsage = float64(metric.Usage.Cpu().MilliValue()) / 1000
				nodeInfo.MemoryUsage = float64(metric.Usage.Memory().Value()) / (1024 * 1024 * 1024)
			}
		}
	}

	var nodeInfos []NodeInfo
	for nodeName, ownerMap := range nodeMap {
		var owners []*OwnerInfo
		totalPods := 0
		for _, owner := range ownerMap {
			owners = append(owners, owner)
			totalPods += owner.PodCount
		}

		sort.Slice(owners, func(i, j int) bool {
			return owners[i].PodCount > owners[j].PodCount
		})

		nodeInfo := nodeStats[nodeName]
		nodeInfo.PodCount = totalPods
		nodeInfo.Owners = owners
		nodeInfos = append(nodeInfos, *nodeInfo)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	for _, nodeInfo := range nodeInfos {
		fmt.Fprintf(w, "\nNode: %s (%d pods)\n", nodeInfo.Name, nodeInfo.PodCount)
		
		cpuUsageStr := "N/A"
		memUsageStr := "N/A"
		if nodeInfo.CPUUsage > 0 {
			cpuUsageStr = fmt.Sprintf("%.2f (%.0f%%)", nodeInfo.CPUUsage, nodeInfo.CPUUsage*100/nodeInfo.CPUCapacity)
		}
		if nodeInfo.MemoryUsage > 0 {
			memUsageStr = fmt.Sprintf("%.2fGi (%.0f%%)", nodeInfo.MemoryUsage, nodeInfo.MemoryUsage*100/nodeInfo.MemoryCapacity)
		}

		fmt.Fprintf(w, "  CPU: %.2f capacity, %.2f (%.0f%%) requests, %.2f (%.0f%%) limits, %s usage\n",
			nodeInfo.CPUCapacity,
			nodeInfo.CPURequests, nodeInfo.CPURequests*100/nodeInfo.CPUCapacity,
			nodeInfo.CPULimits, nodeInfo.CPULimits*100/nodeInfo.CPUCapacity,
			cpuUsageStr)
		
		fmt.Fprintf(w, "  Memory: %.2fGi capacity, %.2fGi (%.0f%%) requests, %.2fGi (%.0f%%) limits, %s usage\n",
			nodeInfo.MemoryCapacity,
			nodeInfo.MemoryRequests, nodeInfo.MemoryRequests*100/nodeInfo.MemoryCapacity,
			nodeInfo.MemoryLimits, nodeInfo.MemoryLimits*100/nodeInfo.MemoryCapacity,
			memUsageStr)

		fmt.Fprintln(w, "  OWNER\tTYPE\tNAMESPACE\tPODS\tCPU REQ\tCPU LIM\tMEM REQ\tMEM LIM")

		for _, owner := range nodeInfo.Owners {
			fmt.Fprintf(w, "  %s\t%s\t%s\t%d\t%.2f\t%.2f\t%.2fGi\t%.2fGi\n",
				owner.Name, owner.Type, owner.Namespace, owner.PodCount,
				owner.CPURequest, owner.CPULimit, owner.MemRequest, owner.MemLimit)
		}
	}

	w.Flush()
	return nil
}

func getPodOwnerFast(pod *corev1.Pod, rsOwnerCache map[string]string) (string, string) {
	for _, owner := range pod.OwnerReferences {
		switch owner.Kind {
		case "ReplicaSet":
			if deploymentName, exists := rsOwnerCache[pod.Namespace+"/"+owner.Name]; exists {
				return deploymentName, "Deployment"
			}
			return owner.Name, "ReplicaSet"
		case "DaemonSet":
			return owner.Name, "DaemonSet"
		case "StatefulSet":
			return owner.Name, "StatefulSet"
		case "Job":
			return owner.Name, "Job"
		default:
			return owner.Name, owner.Kind
		}
	}
	return pod.Name, "Pod"
}
