package k8s

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/HighonAces/swissarmycli/internal/k8s/common"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed cost-estimate.json
var pricingConfigData []byte

type PricingConfig struct {
	EC2Pricing map[string]float64 `json:"ec2_pricing"`
	EBSPricing map[string]float64 `json:"ebs_pricing"`
	LBPricing  map[string]float64 `json:"lb_pricing"`
}

type ClusterCostInfo struct {
	Region        string
	EC2Instances  []EC2Instance
	EBSVolumes    []EBSVolume
	LoadBalancers []LoadBalancer
	TotalCost     float64
}

type EC2Instance struct {
	InstanceType string
	Count        int
	HourlyCost   float64
	MonthlyCost  float64
}

type EBSVolume struct {
	VolumeType  string
	SizeGB      int64
	Count       int
	MonthlyCost float64
}

type LoadBalancer struct {
	Type        string
	Count       int
	HourlyCost  float64
	MonthlyCost float64
}

func loadPricingConfig() (*PricingConfig, error) {
	var config PricingConfig
	if err := json.Unmarshal(pricingConfigData, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func EstimateClusterCost() error {
	clientset, err := common.GetKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	costInfo := &ClusterCostInfo{}

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}
	if len(nodes.Items) > 0 {
		costInfo.Region = nodes.Items[0].Labels["topology.kubernetes.io/region"]
	}

	fmt.Printf("Analyzing cluster in region: %s\n", costInfo.Region)

	if err := getEC2InstancesFromNodes(clientset, costInfo); err != nil {
		return fmt.Errorf("failed to get EC2 instances: %w", err)
	}

	if err := getEBSVolumesFromPVs(clientset, costInfo); err != nil {
		return fmt.Errorf("failed to get EBS volumes: %w", err)
	}

	if err := getLoadBalancersFromServices(clientset, costInfo); err != nil {
		return fmt.Errorf("failed to get load balancers: %w", err)
	}

	if err := calculateCosts(costInfo); err != nil {
		return fmt.Errorf("failed to calculate costs: %w", err)
	}

	printCostEstimation(costInfo)
	return nil
}

func getEC2InstancesFromNodes(clientset *kubernetes.Clientset, costInfo *ClusterCostInfo) error {
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	instanceCounts := make(map[string]int)
	for _, node := range nodes.Items {
		instanceType := node.Labels["node.kubernetes.io/instance-type"]
		if instanceType == "" {
			instanceType = node.Labels["beta.kubernetes.io/instance-type"]
		}
		if instanceType != "" {
			instanceCounts[instanceType]++
		}
	}

	for instanceType, count := range instanceCounts {
		costInfo.EC2Instances = append(costInfo.EC2Instances, EC2Instance{
			InstanceType: instanceType,
			Count:        count,
		})
	}

	return nil
}

func getEBSVolumesFromPVs(clientset *kubernetes.Clientset, costInfo *ClusterCostInfo) error {
	pvs, err := clientset.CoreV1().PersistentVolumes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	scList, err := clientset.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	scToVolumeType := make(map[string]string)
	for _, sc := range scList.Items {
		if sc.Provisioner == "ebs.csi.aws.com" || sc.Provisioner == "kubernetes.io/aws-ebs" {
			volumeType := sc.Parameters["type"]
			if volumeType == "" {
				volumeType = "gp3"
			}
			scToVolumeType[sc.Name] = volumeType
		}
	}

	volumeInfo := make(map[string]int64)
	for _, pv := range pvs.Items {
		if pv.Spec.StorageClassName != "" {
			volumeType := scToVolumeType[pv.Spec.StorageClassName]
			if volumeType != "" {
				sizeGi := pv.Spec.Capacity.Storage().Value() / (1024 * 1024 * 1024)
				volumeInfo[volumeType] += sizeGi
			}
		}
	}

	for volumeType, totalSize := range volumeInfo {
		costInfo.EBSVolumes = append(costInfo.EBSVolumes, EBSVolume{
			VolumeType: volumeType,
			SizeGB:     totalSize,
			Count:      1,
		})
	}

	return nil
}


func getLoadBalancersFromServices(clientset *kubernetes.Clientset, costInfo *ClusterCostInfo) error {
	services, err := clientset.CoreV1().Services("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	lbCounts := make(map[string]int)
	for _, svc := range services.Items {
		if svc.Spec.Type == v1.ServiceTypeLoadBalancer {
			lbType := "classic"
			
			if lbTypeAnnotation, ok := svc.Annotations["service.beta.kubernetes.io/aws-load-balancer-type"]; ok {
				if strings.Contains(lbTypeAnnotation, "nlb") {
					lbType = "network"
				} else if strings.Contains(lbTypeAnnotation, "alb") {
					lbType = "application"
				}
			}
			
			lbCounts[lbType]++
		}
	}

	for lbType, count := range lbCounts {
		costInfo.LoadBalancers = append(costInfo.LoadBalancers, LoadBalancer{
			Type:  lbType,
			Count: count,
		})
	}

	return nil
}

func calculateCosts(costInfo *ClusterCostInfo) error {
	pricing, err := loadPricingConfig()
	if err != nil {
		return fmt.Errorf("failed to load pricing config: %w", err)
	}

	for i := range costInfo.EC2Instances {
		price, ok := pricing.EC2Pricing[costInfo.EC2Instances[i].InstanceType]
		if !ok {
			fmt.Printf("Warning: No price found for %s, skipping\n", costInfo.EC2Instances[i].InstanceType)
			continue
		}
		costInfo.EC2Instances[i].HourlyCost = price
		costInfo.EC2Instances[i].MonthlyCost = price * 730 * float64(costInfo.EC2Instances[i].Count)
		costInfo.TotalCost += costInfo.EC2Instances[i].MonthlyCost
	}

	for i := range costInfo.EBSVolumes {
		price, ok := pricing.EBSPricing[costInfo.EBSVolumes[i].VolumeType]
		if !ok {
			fmt.Printf("Warning: No price found for %s, skipping\n", costInfo.EBSVolumes[i].VolumeType)
			continue
		}
		costInfo.EBSVolumes[i].MonthlyCost = price * float64(costInfo.EBSVolumes[i].SizeGB)
		costInfo.TotalCost += costInfo.EBSVolumes[i].MonthlyCost
	}

	for i := range costInfo.LoadBalancers {
		price, ok := pricing.LBPricing[costInfo.LoadBalancers[i].Type]
		if !ok {
			fmt.Printf("Warning: No price found for %s LB, skipping\n", costInfo.LoadBalancers[i].Type)
			continue
		}
		costInfo.LoadBalancers[i].HourlyCost = price
		costInfo.LoadBalancers[i].MonthlyCost = price * 730 * float64(costInfo.LoadBalancers[i].Count)
		costInfo.TotalCost += costInfo.LoadBalancers[i].MonthlyCost
	}

	return nil
}

func printCostEstimation(costInfo *ClusterCostInfo) {
	fmt.Printf("\n--- Cost Estimation Summary ---\n")
	fmt.Printf("Region: %s\n\n", costInfo.Region)
	
	fmt.Printf("EC2 Instances:\n")
	for _, instance := range costInfo.EC2Instances {
		fmt.Printf("  %s: %d instances - $%.4f/hour - $%.2f/month\n", 
			instance.InstanceType, instance.Count, instance.HourlyCost, instance.MonthlyCost)
	}
	
	fmt.Printf("\nEBS Volumes:\n")
	for _, volume := range costInfo.EBSVolumes {
		fmt.Printf("  %s: %d GB total - $%.2f/month\n", 
			volume.VolumeType, volume.SizeGB, volume.MonthlyCost)
	}
	
	fmt.Printf("\nLoad Balancers:\n")
	for _, lb := range costInfo.LoadBalancers {
		fmt.Printf("  %s: %d - $%.4f/hour - $%.2f/month\n", 
			lb.Type, lb.Count, lb.HourlyCost, lb.MonthlyCost)
	}
	
	fmt.Printf("\nEstimated Monthly Total: $%.2f\n", costInfo.TotalCost)
	fmt.Println("----------------------------------------------------")
}
