package main

import (
	"fmt"
	"os"

	"github.com/HighonAces/swissarmycli/internal/aws"
	"github.com/HighonAces/swissarmycli/internal/k8s"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "swissarmycli",
		Short: "Swiss Army CLI - A multi-purpose CLI tool",
		Long: `Swiss Army CLI is a versatile tool for platform engineering and DevOps tasks.
It provides various utilities for working with Kubernetes, AWS, and more.`,
	}

	// Connect command
	var connectCmd = &cobra.Command{
		Use:   "connect [nodeName]",
		Short: "Connect to an AWS worker node using SSM",
		Long:  `Connect to an AWS worker node in a Kubernetes cluster using AWS Systems Manager (SSM).`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			nodeName := args[0]
			err := aws.ConnectToNode(nodeName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error connecting to node: %v\n", err)
				os.Exit(1)
			}
		},
	}

	//node usage command
	var nodeUsageCmd = &cobra.Command{
		Use:   "node-usage",
		Short: "Display CPU and memory usage of all nodes",
		Long:  `Display CPU and memory requests and limits for all nodes in the Kubernetes cluster.`,
		Run: func(cmd *cobra.Command, args []string) {
			err := k8s.ShowNodeUsage()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error displaying node usage: %v\n", err)
				os.Exit(1)
			}
		},
	}
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(nodeUsageCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error executing command: %v\n", err)
		os.Exit(1)
	}
}
