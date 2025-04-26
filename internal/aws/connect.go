package aws

import (
	"fmt"
)

// ConnectToNode connects to an AWS worker node using SSM
func ConnectToNode(nodeName string) error {
	fmt.Printf("Connecting to node: %s\n", nodeName)

	// TODO: Add code to get the instance ID from the node name
	// This will be implemented later as mentioned
	instanceID := getInstanceIDFromNodeName(nodeName)

	if instanceID == "" {
		return fmt.Errorf("could not find instance ID for node %s", nodeName)
	}

	fmt.Printf("Found instance ID: %s\n", instanceID)

	// Start an SSM session
	return startSSMSession(instanceID)
}

// Placeholder function that will be implemented later
func getInstanceIDFromNodeName(nodeName string) string {
	// This is just a placeholder
	// You'll implement this function later with your specific logic
	fmt.Println("Getting instance ID from node name (placeholder)")
	return ""
}

// startSSMSession starts an SSM session to the specified instance
func startSSMSession(instanceID string) error {
	// Load AWS configuration
	fmt.Printf("Reached startSSMSession %s\n", instanceID)

	return nil
}
