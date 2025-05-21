package aws

import (
	"fmt" // Or other logging if you have a standard
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elbv2"
)

type NLBInfo struct {
	Name    string
	DNSName string
	IPs     []string
}

// ListNLBs fetches NLB information for a given region.
// It returns the NLB's name, DNS name, and the IP addresses of its network interfaces.
func ListNLBs(region string) ([]NLBInfo, error) {
	// Initialize a new AWS session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	// Create an ELBv2 client
	svc := elbv2.New(sess)

	var nlbsInfo []NLBInfo

	// Call DescribeLoadBalancers to get all load balancers
	// We will filter for 'network' type client-side, or check if API supports filtering
	input := &elbv2.DescribeLoadBalancersInput{}

	err = svc.DescribeLoadBalancersPages(input,
		func(page *elbv2.DescribeLoadBalancersOutput, lastPage bool) bool {
			for _, lb := range page.LoadBalancers {
				if lb.Type != nil && *lb.Type == elbv2.LoadBalancerTypeEnumNetwork {
					var currentNLBInfo NLBInfo
					currentNLBInfo.Name = aws.StringValue(lb.LoadBalancerName)
					currentNLBInfo.DNSName = aws.StringValue(lb.DNSName)

					// The IP addresses for an NLB are found in its LoadBalancerAddresses,
					// which are part of the AvailabilityZone information.
					// For NLBs, IPs are directly in LoadBalancerAddresses within each AvailabilityZone.
					// DescribeLoadBalancers already returns this information directly in lb.AvailabilityZones.
					// Each AZ can have multiple LoadBalancerAddress entries, especially if dualstack.
					// We are interested in the IpAddress field of these entries.
					for _, az := range lb.AvailabilityZones {
						for _, addr := range az.LoadBalancerAddresses {
							if addr.IpAddress != nil {
								currentNLBInfo.IPs = append(currentNLBInfo.IPs, aws.StringValue(addr.IpAddress))
							}
							// NLBs can also have PrivateIPv4Address or IPv6Address.
							// The prompt specifically asked for "IPs", so IpAddress (public IP) is the primary target.
							// Depending on requirements, might need to include addr.PrivateIPv4Address or addr.IPv6Address.
						}
					}
					nlbsInfo = append(nlbsInfo, currentNLBInfo)
				}
			}
			return !lastPage // Continue to next page
		})

	if err != nil {
		return nil, fmt.Errorf("failed to describe load balancers: %w", err)
	}

	return nlbsInfo, nil
}
