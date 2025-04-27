package aws

import (
	"context"
	"fmt"
	"github.com/HighonAces/swissarmycli/internal/k8s/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"os/exec"
	"strings"
)

var validUSRegions = map[string]bool{
	"us-east-1": true,
	"us-east-2": true,
	"us-west-1": true,
	"us-west-2": true,
}

// ConnectToNode connects to an AWS worker node using SSM
func ConnectToNode(nodeName string) error {
	fmt.Printf("Connecting to node: %s\n", nodeName)

	// TODO: Add code to get the instance ID from the node name
	// This will be implemented later as mentioned
	instanceID, region := getInstanceIDFromNodeName(nodeName)

	if instanceID == "" {
		return fmt.Errorf("could not find instance ID for node %s", nodeName)
	}

	fmt.Printf("Found instance ID: %s\n", instanceID)
	fmt.Printf("Found region: %s\n", region)

	// Start an SSM session
	return startSSMSession(instanceID, region)
}

// Placeholder function that will be implemented later
func getInstanceIDFromNodeName(nodeName string) (string, string) {
	clientset, err := common.GetKubernetesClient() // Use the new public function
	if err != nil {
		fmt.Println("failed to create Kubernetes client: %w", err)
		return "", ""
	}

	//node object now have all the node related info
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, v1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	providerID := node.Spec.ProviderID
	const prefix = "aws:///"
	if !strings.HasPrefix(providerID, prefix) {
		fmt.Println("invalid providerID format")
		return "", ""
	}
	parts := strings.Split(strings.TrimPrefix(providerID, prefix), "/") // Strip prefix and split the rest

	if len(parts) != 2 {
		fmt.Println("unexpected providerID structure")
		return "", ""
	}
	az := parts[0]         // e.g. "us-west-2a"
	instanceID := parts[1] // e.g. "i-0abc1234def56789"

	if len(az) < 9 {
		fmt.Println("invalid availability zone format")
		return "", ""
	}

	// Take first 9 characters for region
	region := az[:9]

	// Validate against known US regions
	if !validUSRegions[region] {
		fmt.Println("unknown or unsupported region: %s", region)
		return "", ""
	}
	return instanceID, region

}

// startSSMSession starts an SSM session to the specified instance
func startSSMSession(instanceID string, region string) error {
	// Load AWS configuration
	fmt.Printf("Attempting to start SSM session to instance %s in region %s via AWS CLI...\n", instanceID, region)
	// Construct the command to execute
	// Using AWS-StartSSHSession document is common for interactive shells via SSM
	cmd := exec.Command("aws", "ssm", "start-session",
		"--target", instanceID,
		"--region", region,
	)

	// Connect the command's standard input, output, and error streams
	// directly to the Go program's streams. This makes the session interactive.
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		// Provide context about potential issues
		return fmt.Errorf("failed to execute 'aws ssm start-session': %w. \nPossible causes:\n"+
			"  - AWS CLI is not installed or not in PATH.\n"+
			"  - AWS credentials are not configured correctly.\n"+
			"  - Instance '%s' does not exist or is not managed by SSM.\n"+
			"  - SSM Agent is not running on the instance.\n"+
			"  - IAM permissions for SSM StartSession are missing for your user/role.\n"+
			"  - IAM instance profile permissions are missing for the target instance.", err, instanceID)
	}
	return err
}
