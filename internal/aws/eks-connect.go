package aws

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
)

// EKSClusterInfo holds basic information about an EKS cluster.
type EKSClusterInfo struct {
	Name   string
	Region string
}

// usRegionsToSearch defines the AWS regions to scan for EKS clusters.
var usRegionsToSearch = []string{"us-east-1", "us-east-2", "us-west-1", "us-west-2"}

// ConnectToEKSCluster finds an EKS cluster and updates kubeconfig.
func ConnectToEKSCluster(partialName string) error {
	fmt.Printf("Searching for EKS clusters containing '%s' in regions: %s...\n", partialName, strings.Join(usRegionsToSearch, ", "))

	var matchingClusters []EKSClusterInfo
	// Create a base session. We'll override the region for each iteration.
	// This assumes default credential chain or a profile specified via environment.
	// If you add --profile flag to `connect cluster`, you'd pass it here.
	baseSess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		// Config: aws.Config{
		//  // If you want to set a default region for the session object itself,
		//  // but we override it per API call below.
		// },
		// Profile: "your-profile-if-passed-as-flag",
	})
	if err != nil {
		return fmt.Errorf("failed to create base AWS session: %w", err)
	}
	for _, region := range usRegionsToSearch {
		fmt.Printf("Checking region: %s\n", region)
		// It's more efficient to create a new service client per region
		// than creating a new session object every time if only region changes.
		// However, creating a new session with a specific region is also fine.
		regionalSess := baseSess.Copy(&aws.Config{Region: aws.String(region)})
		eksSvc := eks.New(regionalSess)

		input := &eks.ListClustersInput{}
		// Potentially add MaxResults and NextToken for pagination if many clusters exist.
		// For typical use cases, this might not be immediately necessary.

		err := eksSvc.ListClustersPages(input,
			func(page *eks.ListClustersOutput, lastPage bool) bool {
				for _, clusterNamePtr := range page.Clusters {
					if clusterNamePtr != nil {
						clusterName := *clusterNamePtr
						if strings.Contains(strings.ToLower(clusterName), strings.ToLower(partialName)) {
							matchingClusters = append(matchingClusters, EKSClusterInfo{
								Name:   clusterName,
								Region: region,
							})
						}
					}
				}
				return !lastPage // Continue to next page if not the last
			})

		if err != nil {
			// Log error for the region but continue to other regions
			fmt.Fprintf(os.Stderr, "Warning: could not list clusters in region %s: %v\n", region, err)
		}
	}

	if len(matchingClusters) == 0 {
		fmt.Printf("No EKS clusters found matching '%s'.\n", partialName)
		return nil
	}

	var selectedCluster EKSClusterInfo
	if len(matchingClusters) == 1 {
		selectedCluster = matchingClusters[0]
		fmt.Printf("Found one matching cluster: %s (%s)\n", selectedCluster.Name, selectedCluster.Region)
	} else {
		fmt.Println("\nMultiple EKS clusters found. Please select one:")
		for i, cluster := range matchingClusters {
			fmt.Printf("  %d. %s (%s)\n", i+1, cluster.Name, cluster.Region)
		}
		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("Enter number: ")
			inputStr, _ := reader.ReadString('\n')
			inputStr = strings.TrimSpace(inputStr)
			choice, err := strconv.Atoi(inputStr)
			if err != nil || choice < 1 || choice > len(matchingClusters) {
				fmt.Println("Invalid selection. Please enter a number from the list.")
				continue
			}
			selectedCluster = matchingClusters[choice-1]
			break
		}
	}

	fmt.Printf("Updating kubeconfig for cluster: %s in region %s...\n", selectedCluster.Name, selectedCluster.Region)
	return updateKubeconfigForEKS(selectedCluster.Name, selectedCluster.Region)
}

func updateKubeconfigForEKS(clusterName string, region string) error {
	cmd := exec.Command("aws", "eks", "update-kubeconfig",
		"--name", clusterName,
		"--region", region,
		// You might want to add --alias if you prefer specific context names
		// or --kubeconfig if you want to update a non-default kubeconfig file.
	)

	cmd.Stdout = os.Stdout // Show output from aws cli
	cmd.Stderr = os.Stderr // Show errors from aws cli

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run 'aws eks update-kubeconfig' for %s (%s): %w", clusterName, region, err)
	}

	fmt.Printf("Kubeconfig updated successfully for cluster %s (%s).\n", clusterName, region)
	return nil
}
