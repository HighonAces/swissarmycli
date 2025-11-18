package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	awsutils "github.com/HighonAces/swissarmycli/internal/aws"
	"github.com/HighonAces/swissarmycli/internal/k8s/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type ClusterSnapshot struct {
	Timestamp      time.Time                `json:"timestamp" yaml:"timestamp"`
	Summary        ClusterSummary           `json:"summary" yaml:"summary"`
	Dump           ClusterDump              `json:"dump" yaml:"dump"`
}

type ClusterSummary struct {
	Nodes          []NodeSummary            `json:"nodes" yaml:"nodes"`
	Deployments    []DeploymentSummary      `json:"deployments" yaml:"deployments"`
	NonRunningPods []PodSummary             `json:"non_running_pods" yaml:"non_running_pods"`
	HelmReleases   []HelmRelease            `json:"helm_releases" yaml:"helm_releases"`
	PVs            []PVSummary              `json:"persistent_volumes" yaml:"persistent_volumes"`
	PVCs           []PVCSummary             `json:"persistent_volume_claims" yaml:"persistent_volume_claims"`
	StorageClasses []StorageClassSummary    `json:"storage_classes" yaml:"storage_classes"`
	ENIConfigs     []ENIConfigSummary       `json:"eni_configs" yaml:"eni_configs"`
	SubnetInfo     []SubnetInfo             `json:"subnet_info" yaml:"subnet_info"`
	NodeSubnets    []awsutils.NodeSubnetInfo `json:"node_subnets" yaml:"node_subnets"`
}

type ClusterDump struct {
	Nodes          []corev1.Node            `json:"nodes" yaml:"nodes"`
	Services       []corev1.Service         `json:"services" yaml:"services"`
	Deployments    []appsv1.Deployment      `json:"deployments" yaml:"deployments"`
	DaemonSets     []appsv1.DaemonSet       `json:"daemonsets" yaml:"daemonsets"`
	StatefulSets   []appsv1.StatefulSet     `json:"statefulsets" yaml:"statefulsets"`
	Pods           []corev1.Pod             `json:"pods" yaml:"pods"`
	PVCs           []corev1.PersistentVolumeClaim `json:"pvcs" yaml:"pvcs"`
	PVs            []corev1.PersistentVolume `json:"pvs" yaml:"pvs"`
	StorageClasses []storagev1.StorageClass `json:"storageclasses" yaml:"storageclasses"`
	ENIConfigs     []unstructured.Unstructured `json:"eni_configs" yaml:"eni_configs"`
}

type NodeSummary struct {
	Name   string `json:"name" yaml:"name"`
	Ready  bool   `json:"ready" yaml:"ready"`
	Status string `json:"status" yaml:"status"`
}

type DeploymentSummary struct {
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Replicas  string `json:"replicas" yaml:"replicas"`
}

type PodSummary struct {
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Phase     string `json:"phase" yaml:"phase"`
	Node      string `json:"node" yaml:"node"`
}

type PVSummary struct {
	Name   string `json:"name" yaml:"name"`
	Size   string `json:"size" yaml:"size"`
	Status string `json:"status" yaml:"status"`
}

type PVCSummary struct {
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Size      string `json:"size" yaml:"size"`
	Status    string `json:"status" yaml:"status"`
}

type StorageClassSummary struct {
	Name        string `json:"name" yaml:"name"`
	Provisioner string `json:"provisioner" yaml:"provisioner"`
}

type ENIConfigSummary struct {
	Name             string `json:"name" yaml:"name"`
	SubnetID         string `json:"subnet_id" yaml:"subnet_id"`
	AvailabilityZone string `json:"availability_zone" yaml:"availability_zone"`
	AvailableIPs     int    `json:"available_ips" yaml:"available_ips"`
}

type SubnetInfo struct {
	SubnetID     string `json:"subnet_id" yaml:"subnet_id"`
	CIDR         string `json:"cidr" yaml:"cidr"`
	AvailableIPs int    `json:"available_ips" yaml:"available_ips"`
	Type         string `json:"type" yaml:"type"` // "primary" or "secondary"
}

type HelmRelease struct {
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Chart     string `json:"chart" yaml:"chart"`
	Version   string `json:"version" yaml:"version"`
	Status    string `json:"status" yaml:"status"`
}

func GetClusterSnapshot(format string) error {
	clientset, err := common.GetKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	fmt.Println("Collecting cluster snapshot...")

	snapshot := ClusterSnapshot{
		Timestamp: time.Now(),
		Summary:   ClusterSummary{},
		Dump:      ClusterDump{},
	}

	ctx := context.TODO()

	// Collect nodes
	fmt.Print("Collecting nodes... ")
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to get nodes: %w", err)
	}
	snapshot.Dump.Nodes = nodes.Items
	fmt.Printf("✓ (%d)\n", len(nodes.Items))

	// Collect services
	fmt.Print("Collecting services... ")
	services, err := clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to get services: %w", err)
	}
	snapshot.Dump.Services = services.Items
	fmt.Printf("✓ (%d)\n", len(services.Items))

	// Collect deployments
	fmt.Print("Collecting deployments... ")
	deployments, err := clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployments: %w", err)
	}
	snapshot.Dump.Deployments = deployments.Items
	fmt.Printf("✓ (%d)\n", len(deployments.Items))

	// Collect daemonsets
	fmt.Print("Collecting daemonsets... ")
	daemonsets, err := clientset.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to get daemonsets: %w", err)
	}
	snapshot.Dump.DaemonSets = daemonsets.Items
	fmt.Printf("✓ (%d)\n", len(daemonsets.Items))

	// Collect statefulsets
	fmt.Print("Collecting statefulsets... ")
	statefulsets, err := clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to get statefulsets: %w", err)
	}
	snapshot.Dump.StatefulSets = statefulsets.Items
	fmt.Printf("✓ (%d)\n", len(statefulsets.Items))

	// Collect pods
	fmt.Print("Collecting pods... ")
	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pods: %w", err)
	}
	snapshot.Dump.Pods = pods.Items
	fmt.Printf("✓ (%d)\n", len(pods.Items))

	// Collect PVCs
	fmt.Print("Collecting PVCs... ")
	pvcs, err := clientset.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to get PVCs: %w", err)
	}
	snapshot.Dump.PVCs = pvcs.Items
	fmt.Printf("✓ (%d)\n", len(pvcs.Items))

	// Collect PVs
	fmt.Print("Collecting PVs... ")
	pvs, err := clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to get PVs: %w", err)
	}
	snapshot.Dump.PVs = pvs.Items
	fmt.Printf("✓ (%d)\n", len(pvs.Items))

	// Collect storage classes
	fmt.Print("Collecting storage classes... ")
	storageClasses, err := clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to get storage classes: %w", err)
	}
	snapshot.Dump.StorageClasses = storageClasses.Items
	fmt.Printf("✓ (%d)\n", len(storageClasses.Items))

	// Collect ENIConfigs
	fmt.Print("Collecting ENIConfigs... ")
	eniConfigs, err := getENIConfigs()
	if err != nil {
		fmt.Printf("⚠ (skipped: %v)\n", err)
	} else {
		snapshot.Dump.ENIConfigs = eniConfigs
		fmt.Printf("✓ (%d)\n", len(eniConfigs))
	}

	// Try to collect Helm releases (optional)
	fmt.Print("Collecting Helm releases... ")
	helmReleases, err := getHelmReleases(clientset)
	if err != nil {
		fmt.Printf("⚠ (skipped: %v)\n", err)
	} else {
		snapshot.Summary.HelmReleases = helmReleases
		fmt.Printf("✓ (%d)\n", len(helmReleases))
	}

	// Build summary
	fmt.Print("Building summary... ")
	buildSummary(&snapshot)
	fmt.Println("✓")

	// Get node subnet information
	fmt.Print("Collecting node subnet info... ")
	nodeSubnetInfo := awsutils.GetNodeSubnetInfo(snapshot.Dump.Nodes)
	snapshot.Summary.NodeSubnets = nodeSubnetInfo
	fmt.Printf("✓ (%d)\n", len(nodeSubnetInfo))

	// Get cluster name from kubeconfig context
	clusterName, err := getClusterName()
	if err != nil {
		fmt.Printf("Warning: could not get cluster name: %v, using 'unknown'\n", err)
		clusterName = "unknown"
	}

	// Generate filename with cluster name and timestamp
	timestamp := time.Now().Format("20060102-150405")
	var filename string
	var content []byte

	switch format {
	case "yaml", "yml":
		filename = fmt.Sprintf("%s-snapshot-%s.yaml", clusterName, timestamp)
		content, err = marshalSnapshotYAML(snapshot)
		if err != nil {
			return fmt.Errorf("failed to marshal to YAML: %w", err)
		}
	case "txt":
		filename = fmt.Sprintf("%s-snapshot-%s.txt", clusterName, timestamp)
		content = []byte(formatSnapshotAsText(snapshot))
	default:
		return fmt.Errorf("unsupported format: %s (supported: yaml, txt)", format)
	}

	// Write to file
	err = os.WriteFile(filename, content, 0644)
	if err != nil {
		return fmt.Errorf("failed to write snapshot to file: %w", err)
	}

	absPath, _ := filepath.Abs(filename)
	fmt.Printf("\n✅ Cluster snapshot saved to: %s\n", absPath)
	return nil
}

func getHelmReleases(clientset *kubernetes.Clientset) ([]HelmRelease, error) {
	// Try to get Helm releases from secrets in all namespaces
	secrets, err := clientset.CoreV1().Secrets("").List(context.TODO(), metav1.ListOptions{
		LabelSelector: "owner=helm",
	})
	if err != nil {
		return nil, err
	}

	var releases []HelmRelease
	for _, secret := range secrets.Items {
		if secret.Type == "helm.sh/release.v1" {
			release := HelmRelease{
				Name:      secret.Labels["name"],
				Namespace: secret.Namespace,
				Status:    secret.Labels["status"],
				Version:   secret.Labels["version"],
			}
			releases = append(releases, release)
		}
	}

	return releases, nil
}

func buildSummary(snapshot *ClusterSnapshot) {
	// Build node summary
	for _, node := range snapshot.Dump.Nodes {
		summary := NodeSummary{
			Name:   node.Name,
			Ready:  getNodeReadyStatus(node) == "True",
			Status: getNodeReadyStatus(node),
		}
		snapshot.Summary.Nodes = append(snapshot.Summary.Nodes, summary)
	}

	// Build deployment summary
	for _, dep := range snapshot.Dump.Deployments {
		replicas := "0/0"
		if dep.Spec.Replicas != nil {
			replicas = fmt.Sprintf("%d/%d", dep.Status.ReadyReplicas, *dep.Spec.Replicas)
		}
		summary := DeploymentSummary{
			Name:      dep.Name,
			Namespace: dep.Namespace,
			Replicas:  replicas,
		}
		snapshot.Summary.Deployments = append(snapshot.Summary.Deployments, summary)
	}

	// Build non-running pods summary
	for _, pod := range snapshot.Dump.Pods {
		if pod.Status.Phase != corev1.PodRunning {
			summary := PodSummary{
				Name:      pod.Name,
				Namespace: pod.Namespace,
				Phase:     string(pod.Status.Phase),
				Node:      pod.Spec.NodeName,
			}
			snapshot.Summary.NonRunningPods = append(snapshot.Summary.NonRunningPods, summary)
		}
	}

	// Build PV summary
	for _, pv := range snapshot.Dump.PVs {
		size := "Unknown"
		if storage, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
			size = storage.String()
		}
		summary := PVSummary{
			Name:   pv.Name,
			Size:   size,
			Status: string(pv.Status.Phase),
		}
		snapshot.Summary.PVs = append(snapshot.Summary.PVs, summary)
	}

	// Build PVC summary
	for _, pvc := range snapshot.Dump.PVCs {
		size := "Unknown"
		if storage, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			size = storage.String()
		}
		summary := PVCSummary{
			Name:      pvc.Name,
			Namespace: pvc.Namespace,
			Size:      size,
			Status:    string(pvc.Status.Phase),
		}
		snapshot.Summary.PVCs = append(snapshot.Summary.PVCs, summary)
	}

	// Build storage class summary
	for _, sc := range snapshot.Dump.StorageClasses {
		summary := StorageClassSummary{
			Name:        sc.Name,
			Provisioner: sc.Provisioner,
		}
		snapshot.Summary.StorageClasses = append(snapshot.Summary.StorageClasses, summary)
	}

	// Build ENIConfig and subnet summary
	eniConfigSummary, subnetInfo := buildENIConfigAndSubnetSummary(snapshot.Dump.ENIConfigs, snapshot.Dump.Pods)
	snapshot.Summary.ENIConfigs = eniConfigSummary
	snapshot.Summary.SubnetInfo = subnetInfo
}

func formatSnapshotAsText(snapshot ClusterSnapshot) string {
	var content string
	
	content += fmt.Sprintf("=== CLUSTER SNAPSHOT ===\n")
	content += fmt.Sprintf("Timestamp: %s\n\n", snapshot.Timestamp.Format("2006-01-02 15:04:05 MST"))

	content += fmt.Sprintf("=== SUMMARY ===\n\n")

	content += fmt.Sprintf("=== NODES (%d) ===\n", len(snapshot.Summary.Nodes))
	for _, node := range snapshot.Summary.Nodes {
		content += fmt.Sprintf("- %s (Ready: %t)\n", node.Name, node.Ready)
	}
	content += "\n"

	content += fmt.Sprintf("=== DEPLOYMENTS (%d) ===\n", len(snapshot.Summary.Deployments))
	for _, dep := range snapshot.Summary.Deployments {
		content += fmt.Sprintf("- %s/%s (Replicas: %s)\n", dep.Namespace, dep.Name, dep.Replicas)
	}
	content += "\n"

	if len(snapshot.Summary.NonRunningPods) > 0 {
		content += fmt.Sprintf("=== NON-RUNNING PODS (%d) ===\n", len(snapshot.Summary.NonRunningPods))
		for _, pod := range snapshot.Summary.NonRunningPods {
			content += fmt.Sprintf("- %s/%s (Phase: %s, Node: %s)\n", pod.Namespace, pod.Name, pod.Phase, pod.Node)
		}
		content += "\n"
	}

	if len(snapshot.Summary.HelmReleases) > 0 {
		content += fmt.Sprintf("=== HELM RELEASES (%d) ===\n", len(snapshot.Summary.HelmReleases))
		for _, release := range snapshot.Summary.HelmReleases {
			content += fmt.Sprintf("- %s/%s (Status: %s, Version: %s)\n", release.Namespace, release.Name, release.Status, release.Version)
		}
		content += "\n"
	}

	content += fmt.Sprintf("=== PERSISTENT VOLUMES (%d) ===\n", len(snapshot.Summary.PVs))
	for _, pv := range snapshot.Summary.PVs {
		content += fmt.Sprintf("- %s (Status: %s, Size: %s)\n", pv.Name, pv.Status, pv.Size)
	}
	content += "\n"

	content += fmt.Sprintf("=== PERSISTENT VOLUME CLAIMS (%d) ===\n", len(snapshot.Summary.PVCs))
	for _, pvc := range snapshot.Summary.PVCs {
		content += fmt.Sprintf("- %s/%s (Status: %s, Size: %s)\n", pvc.Namespace, pvc.Name, pvc.Status, pvc.Size)
	}
	content += "\n"

	content += fmt.Sprintf("=== STORAGE CLASSES (%d) ===\n", len(snapshot.Summary.StorageClasses))
	for _, sc := range snapshot.Summary.StorageClasses {
		content += fmt.Sprintf("- %s (Provisioner: %s)\n", sc.Name, sc.Provisioner)
	}
	content += "\n"

	if len(snapshot.Summary.ENIConfigs) > 0 {
		content += fmt.Sprintf("=== ENI CONFIGS (%d) ===\n", len(snapshot.Summary.ENIConfigs))
		for _, eni := range snapshot.Summary.ENIConfigs {
			content += fmt.Sprintf("- %s (Subnet: %s, AZ: %s, Available IPs: %d)\n", eni.Name, eni.SubnetID, eni.AvailabilityZone, eni.AvailableIPs)
		}
		content += "\n"
	}

	if len(snapshot.Summary.SubnetInfo) > 0 {
		content += fmt.Sprintf("=== SUBNET INFORMATION (%d) ===\n", len(snapshot.Summary.SubnetInfo))
		for _, subnet := range snapshot.Summary.SubnetInfo {
			content += fmt.Sprintf("- %s (%s) - CIDR: %s, Available IPs: %d, Type: %s\n", subnet.SubnetID, subnet.Type, subnet.CIDR, subnet.AvailableIPs, subnet.Type)
		}
		content += "\n"
	}

	if len(snapshot.Summary.NodeSubnets) > 0 {
		content += fmt.Sprintf("=== NODE SUBNETS (%d) ===\n", len(snapshot.Summary.NodeSubnets))
		for _, nodeSubnet := range snapshot.Summary.NodeSubnets {
			content += fmt.Sprintf("- %s (Available IPs: %d, Nodes: %d)\n", nodeSubnet.SubnetID, nodeSubnet.AvailableIPs, nodeSubnet.NodeCount)
		}
		content += "\n"
	}

	content += fmt.Sprintf("=== DUMP ===\n\n")
	content += fmt.Sprintf("Full cluster resource dump including ENIConfigs available in YAML format.\n")
	content += fmt.Sprintf("Use --format yaml to get complete resource definitions.\n")

	return content
}

func getClusterName() (string, error) {
	// Get from kubeconfig context
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		return "", err
	}

	currentContext := rawConfig.CurrentContext
	if context, exists := rawConfig.Contexts[currentContext]; exists {
		if context.Cluster != "" {
			// Extract cluster name from ARN if it's an ARN
			clusterIdentifier := context.Cluster
			if strings.HasPrefix(clusterIdentifier, "arn:aws:eks:") {
				// Parse ARN: arn:aws:eks:region:account:cluster/cluster-name
				parts := strings.Split(clusterIdentifier, "/")
				if len(parts) > 1 {
					return parts[len(parts)-1], nil
				}
			}
			return clusterIdentifier, nil
		}
	}

	return "unknown", nil
}

func marshalSnapshotYAML(snapshot ClusterSnapshot) ([]byte, error) {
	// Marshal each section separately to control order
	var result strings.Builder
	
	// Timestamp first
	timestampYAML, _ := yaml.Marshal(map[string]interface{}{"timestamp": snapshot.Timestamp})
	result.Write(timestampYAML)
	
	// Summary section
	summaryYAML, _ := yaml.Marshal(map[string]interface{}{"summary": snapshot.Summary})
	result.Write(summaryYAML)
	
	// Dump section at the end
	dumpYAML, _ := yaml.Marshal(map[string]interface{}{"dump": snapshot.Dump})
	result.Write(dumpYAML)
	
	return []byte(result.String()), nil
}

func getENIConfigs() ([]unstructured.Unstructured, error) {
	// Get kubeconfig
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	// Define ENIConfig GVR
	eniConfigGVR := schema.GroupVersionResource{
		Group:    "crd.k8s.amazonaws.com",
		Version:  "v1alpha1",
		Resource: "eniconfigs",
	}

	// Get ENIConfigs
	eniConfigList, err := dynamicClient.Resource(eniConfigGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return eniConfigList.Items, nil
}

func buildENIConfigAndSubnetSummary(eniConfigs []unstructured.Unstructured, pods []corev1.Pod) ([]ENIConfigSummary, []SubnetInfo) {
	var eniConfigSummary []ENIConfigSummary
	var subnetInfo []SubnetInfo
	subnetMap := make(map[string]bool)

	// Create AWS session
	sess, err := session.NewSession()
	if err != nil {
		fmt.Printf("Warning: could not create AWS session: %v\n", err)
		return eniConfigSummary, subnetInfo
	}

	ec2Svc := ec2.New(sess)

	// Process ENIConfigs
	for _, eniConfig := range eniConfigs {
		name := eniConfig.GetName()
		spec, found, _ := unstructured.NestedMap(eniConfig.Object, "spec")
		if !found {
			continue
		}

		subnetID, _, _ := unstructured.NestedString(spec, "subnet")
		az, _, _ := unstructured.NestedString(spec, "availabilityZone")

		if subnetID != "" {
			subnetMap[subnetID] = true
			availableIPs := awsutils.GetSubnetAvailableIPsWithRegion(name, subnetID)

			eniConfigSummary = append(eniConfigSummary, ENIConfigSummary{
				Name:             name,
				SubnetID:         subnetID,
				AvailabilityZone: az,
				AvailableIPs:     availableIPs,
			})
		}
	}

	// Find secondary subnets from pod IPs
	secondarySubnets := awsutils.FindSecondarySubnets(pods, ec2Svc)
	for subnetID := range secondarySubnets {
		if !subnetMap[subnetID] {
			subnetMap[subnetID] = true
		}
	}

	// Get subnet information
	for subnetID := range subnetMap {
		subnetDetails := awsutils.GetSubnetDetails(ec2Svc, subnetID)
		if subnetDetails != nil {
			subnetType := "primary"
			if secondarySubnets[subnetID] {
				subnetType = "secondary"
			}

			subnetInfo = append(subnetInfo, SubnetInfo{
				SubnetID:     subnetID,
				CIDR:         *subnetDetails.CidrBlock,
				AvailableIPs: int(*subnetDetails.AvailableIpAddressCount),
				Type:         subnetType,
			})
		}
	}

	return eniConfigSummary, subnetInfo
}



func getNodeReadyStatus(node corev1.Node) string {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			if condition.Status == corev1.ConditionTrue {
				return "True"
			}
			return "False"
		}
	}
	return "Unknown"
}