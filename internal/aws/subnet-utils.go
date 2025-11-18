package aws

import (
	"fmt"
	"net"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	corev1 "k8s.io/api/core/v1"
)

type NodeSubnetInfo struct {
	SubnetID     string   `json:"subnet_id" yaml:"subnet_id"`
	AvailableIPs int      `json:"available_ips" yaml:"available_ips"`
	NodeCount    int      `json:"node_count" yaml:"node_count"`
	NodeNames    []string `json:"node_names" yaml:"node_names"`
}

func GetSubnetAvailableIPsWithRegion(eniConfigName, subnetID string) int {
	region := extractRegionFromName(eniConfigName)
	if region == "" {
		fmt.Printf("Warning: could not extract region from ENIConfig name: %s\n", eniConfigName)
		return 0
	}

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		fmt.Printf("Warning: could not create AWS session for region %s: %v\n", region, err)
		return 0
	}

	ec2Svc := ec2.New(sess)
	result, err := ec2Svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
		SubnetIds: []*string{aws.String(subnetID)},
	})
	if err != nil {
		fmt.Printf("Warning: could not describe subnet %s in region %s: %v\n", subnetID, region, err)
		return 0
	}
	if len(result.Subnets) == 0 {
		fmt.Printf("Warning: subnet %s not found in region %s\n", subnetID, region)
		return 0
	}
	return int(*result.Subnets[0].AvailableIpAddressCount)
}

func GetSubnetDetails(ec2Svc *ec2.EC2, subnetID string) *ec2.Subnet {
	result, err := ec2Svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
		SubnetIds: []*string{aws.String(subnetID)},
	})
	if err != nil || len(result.Subnets) == 0 {
		return nil
	}
	return result.Subnets[0]
}

func FindSecondarySubnets(pods []corev1.Pod, ec2Svc *ec2.EC2) map[string]bool {
	secondarySubnets := make(map[string]bool)
	podIPs := make(map[string]bool)

	for _, pod := range pods {
		if pod.Status.PodIP != "" {
			podIPs[pod.Status.PodIP] = true
		}
		for _, ip := range pod.Status.PodIPs {
			if ip.IP != "" {
				podIPs[ip.IP] = true
			}
		}
	}

	result, err := ec2Svc.DescribeSubnets(&ec2.DescribeSubnetsInput{})
	if err != nil {
		return secondarySubnets
	}

	for _, subnet := range result.Subnets {
		if subnet.CidrBlock == nil {
			continue
		}

		_, cidr, err := net.ParseCIDR(*subnet.CidrBlock)
		if err != nil {
			continue
		}

		for podIP := range podIPs {
			ip := net.ParseIP(podIP)
			if ip != nil && cidr.Contains(ip) {
				if isSecondarySubnet(subnet) {
					secondarySubnets[*subnet.SubnetId] = true
				}
			}
		}
	}

	return secondarySubnets
}

func GetNodeSubnetInfo(nodes []corev1.Node) []NodeSubnetInfo {
	// Group nodes by region and collect unique subnets
	nodesByRegion := make(map[string][]corev1.Node)
	for _, node := range nodes {
		region := extractRegionFromProviderID(node.Spec.ProviderID)
		if region != "" {
			nodesByRegion[region] = append(nodesByRegion[region], node)
		}
	}

	subnetInfoMap := make(map[string]*NodeSubnetInfo)

	// Process each region
	for region, regionNodes := range nodesByRegion {
		sess, err := session.NewSession(&aws.Config{
			Region: aws.String(region),
		})
		if err != nil {
			fmt.Printf("Warning: could not create AWS session for region %s: %v\n", region, err)
			continue
		}

		ec2Svc := ec2.New(sess)
		
		// Get instance IDs and build node-instance mapping
		var instanceIDs []*string
		nodeInstanceMap := make(map[string]string)
		
		for _, node := range regionNodes {
			instanceID := extractInstanceIDFromProviderID(node.Spec.ProviderID)
			if instanceID != "" {
				instanceIDs = append(instanceIDs, aws.String(instanceID))
				nodeInstanceMap[instanceID] = node.Name
			}
		}

		if len(instanceIDs) == 0 {
			continue
		}

		// Describe instances to get subnet information
		result, err := ec2Svc.DescribeInstances(&ec2.DescribeInstancesInput{
			InstanceIds: instanceIDs,
		})
		if err != nil {
			fmt.Printf("Warning: could not describe instances in region %s: %v\n", region, err)
			continue
		}

		// Collect unique subnets and their nodes
		subnetNodes := make(map[string][]string)
		for _, reservation := range result.Reservations {
			for _, instance := range reservation.Instances {
				if instance.InstanceId != nil && instance.SubnetId != nil {
					instanceID := *instance.InstanceId
					subnetID := *instance.SubnetId
					nodeName := nodeInstanceMap[instanceID]
					
					subnetNodes[subnetID] = append(subnetNodes[subnetID], nodeName)
				}
			}
		}

		// Get subnet details for unique subnets
		var uniqueSubnetIDs []*string
		for subnetID := range subnetNodes {
			uniqueSubnetIDs = append(uniqueSubnetIDs, aws.String(subnetID))
		}

		if len(uniqueSubnetIDs) > 0 {
			subnetResult, err := ec2Svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
				SubnetIds: uniqueSubnetIDs,
			})
			if err == nil {
				for _, subnet := range subnetResult.Subnets {
					if subnet.SubnetId != nil && subnet.AvailableIpAddressCount != nil {
						subnetID := *subnet.SubnetId
						nodes := subnetNodes[subnetID]
						
						subnetInfoMap[subnetID] = &NodeSubnetInfo{
							SubnetID:     subnetID,
							AvailableIPs: int(*subnet.AvailableIpAddressCount),
							NodeCount:    len(nodes),
							NodeNames:    nodes,
						}
					}
				}
			}
		}
	}

	// Convert map to slice
	var nodeSubnetInfo []NodeSubnetInfo
	for _, info := range subnetInfoMap {
		nodeSubnetInfo = append(nodeSubnetInfo, *info)
	}

	return nodeSubnetInfo
}

func extractRegionFromName(name string) string {
	if len(name) >= 9 {
		regionWithAZ := name
		if len(regionWithAZ) > 0 {
			return regionWithAZ[:len(regionWithAZ)-1]
		}
	}

	if len(name) > 2 {
		parts := strings.Split(name, "-")
		if len(parts) >= 3 {
			return strings.Join(parts[0:3], "-")
		}
	}

	return ""
}

func extractRegionFromProviderID(providerID string) string {
	// ProviderID format: aws:///us-west-2a/i-1234567890abcdef0
	if strings.HasPrefix(providerID, "aws:///") {
		parts := strings.Split(providerID, "/")
		if len(parts) >= 4 {
			az := parts[3]
			if len(az) > 1 {
				return az[:len(az)-1] // Remove AZ suffix
			}
		}
	}
	return ""
}

func extractInstanceIDFromProviderID(providerID string) string {
	// ProviderID format: aws:///us-west-2a/i-1234567890abcdef0
	if strings.HasPrefix(providerID, "aws:///") {
		parts := strings.Split(providerID, "/")
		if len(parts) >= 5 {
			return parts[4]
		}
	}
	return ""
}

func getSubnetAvailableIPs(ec2Svc *ec2.EC2, subnetID string) int {
	result, err := ec2Svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
		SubnetIds: []*string{aws.String(subnetID)},
	})
	if err != nil || len(result.Subnets) == 0 {
		return 0
	}
	return int(*result.Subnets[0].AvailableIpAddressCount)
}

func isSecondarySubnet(subnet *ec2.Subnet) bool {
	for _, tag := range subnet.Tags {
		if tag.Key != nil && tag.Value != nil {
			key := strings.ToLower(*tag.Key)
			value := strings.ToLower(*tag.Value)
			
			if strings.Contains(key, "secondary") || strings.Contains(value, "secondary") ||
			   strings.Contains(key, "pod") || strings.Contains(value, "pod") ||
			   strings.Contains(value, "private-with-egress") {
				return true
			}
		}
	}
	return false
}