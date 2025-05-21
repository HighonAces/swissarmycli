package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/HighonAces/swissarmycli/internal/aws"
	"github.com/HighonAces/swissarmycli/internal/k8s"
	"github.com/HighonAces/swissarmycli/internal/validator"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "swissarmycli",
		Short: "Swiss Army CLI - A multi-purpose CLI tool",
		Long: `Swiss Army CLI is a versatile tool for platform engineering and DevOps tasks.
It provides various utilities for working with Kubernetes, AWS, and more.`,
	}

	// --- Parent Connect command ---
	var connectCmd = &cobra.Command{
		Use:     "connect",
		Short:   "Connect to AWS resources (nodes, EKS clusters)",
		Long:    `Provides subcommands to connect to different AWS resources like EC2 instances (nodes) or EKS clusters.`,
		Aliases: []string{"con"},
		// If no subcommand is given, Cobra will show help for connectCmd
	}

	// --- Connect Node subcommand ---
	var connectNodeCmd = &cobra.Command{
		Use:     "node [nodeName]",
		Short:   "Connect to an AWS worker node using SSM",
		Long:    `Connect to an AWS worker node in a Kubernetes cluster using AWS Systems Manager (SSM).`,
		Aliases: []string{"n", "nd"},
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			nodeName := args[0]
			err := aws.ConnectToNode(nodeName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error connecting to node: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// --- Connect Cluster subcommand ---
	var connectClusterCmd = &cobra.Command{
		Use:   "cluster [partial-cluster-name]",
		Short: "Connect to an EKS cluster by updating kubeconfig",
		Long: `Searches for EKS clusters across US regions (us-east-1, us-east-2, us-west-1, us-west-2)
matching the partial name and updates kubeconfig for the selected cluster.`,
		Aliases: []string{"c", "cl", "eks"},
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			partialName := args[0]
			// Get flags if any are added to this command in the future (e.g., specific profile)
			// For now, we assume the global AWS config/profile is used by the aws.ConnectToEKSCluster function.
			// String flags can be retrieved using: profile, _ := cmd.Flags().GetString("profile")

			err := aws.ConnectToEKSCluster(partialName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error connecting to EKS cluster: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// Add subcommands to connectCmd
	connectCmd.AddCommand(connectNodeCmd)
	connectCmd.AddCommand(connectClusterCmd)

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

	// --- ASG Status command ---
	// Declare variables to hold flag values for asg-status
	var asgRegion string
	var asgProfile string
	var asgRefreshInterval int // Renamed from 'refresh' for clarity
	var asgStream bool         // Variable to hold the stream flag value

	var asgStatusCmd = &cobra.Command{
		Use:   "asg-status [ASG_NAME]",
		Short: "Check or monitor the status of an AWS Auto Scaling Group", // Updated Short description
		Long: `Checks the current status of an AWS Auto Scaling Group.
Optionally use the --stream flag to launch an interactive terminal dashboard
to monitor the ASG, showing instances, states, and activities in real-time.`, // Updated Long description
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			asgName := args[0]

			// Use the variables linked to the flags directly
			options := aws.MonitorOptions{
				RefreshInterval: asgRefreshInterval,
				Region:          asgRegion,
				Profile:         asgProfile,
			}

			// Check the boolean variable linked to the --stream flag
			if asgStream {
				fmt.Printf("Starting ASG monitor stream for '%s' (Region: %s, Profile: %s, Interval: %ds)...\n",
					asgName, options.Region, options.Profile, options.RefreshInterval)
				err := aws.Monitor(asgName, options) // Call the streaming monitor function
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error running monitor stream: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("ASG monitor stopped.")
			} else {
				fmt.Printf("Checking current status for ASG '%s' (Region: %s, Profile: %s)...\n",
					asgName, options.Region, options.Profile)
				err := aws.OnlyStatus(asgName, options) // Call the non-streaming status function
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error checking ASG status: %v\n", err)
					os.Exit(1)
				}
			}
		},
	}

	// --- Define flags for asg-status ---
	// Flag for Region
	asgStatusCmd.Flags().StringVarP(&asgRegion, "region", "r", "", "AWS region (optional, uses default configuration if not specified)")
	// Flag for Profile
	asgStatusCmd.Flags().StringVarP(&asgProfile, "profile", "p", "", "AWS profile name (optional, uses default configuration if not specified)")
	// Flag for Refresh Interval (only relevant for --stream mode) - Renamed flag to 'interval' for consistency
	asgStatusCmd.Flags().IntVarP(&asgRefreshInterval, "interval", "i", 5, "Refresh interval in seconds (used with --stream)")
	// Flag for Streaming - THIS IS THE FIX
	asgStatusCmd.Flags().BoolVarP(&asgStream, "stream", "s", false, "Launch interactive monitor stream instead of just checking status once")

	// --- Validate command ---
	var validateCmd = &cobra.Command{
		Use:   "validate [filepath]",
		Short: "Validate the syntax of a file (e.g., YAML)",
		Long:  `Validates the syntax of a specified file. Currently supports YAML.`,
		Args:  cobra.ExactArgs(1), // Requires exactly one argument: the filepath
		Run: func(cmd *cobra.Command, args []string) {
			filePath := args[0]
			fmt.Printf("Validating YAML file: %s\n", filePath)
			err := validator.ValidateYAMLFile(filePath)
			if err != nil {
				// The error from yaml.v3 often includes line numbers
				fmt.Fprintf(os.Stderr, "Validation Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("'%s' is a valid YAML file.\n", filePath)
		},
	}
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(nodeUsageCmd)
	rootCmd.AddCommand(asgStatusCmd)
	rootCmd.AddCommand(validateCmd)

	// --- Parent Get command ---
	var getCmd = &cobra.Command{
		Use:   "get",
		Short: "Get AWS resource information",
		Long:  `Provides subcommands to retrieve information about various AWS resources.`,
	}

	// --- Get NLB subcommand ---
	var nlbRegion string // Variable to hold the region flag for nlbCmd
	var nlbCmd = &cobra.Command{
		Use:   "nlb",
		Short: "List Network Load Balancers (NLBs) and their details",
		Long:  `Fetches and displays NLB names, DNS addresses, and their associated IP addresses for a given AWS region.`,
		Run: func(cmd *cobra.Command, args []string) {
			// nlbRegion is already populated by Cobra due to StringVarP
			// No need to call cmd.Flags().GetString("region") if using the global nlbRegion variable

			nlbs, err := aws.ListNLBs(nlbRegion)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error fetching NLBs: %v\n", err)
				os.Exit(1)
			}

			if len(nlbs) == 0 {
				fmt.Printf("No Network Load Balancers found in region %s\n", nlbRegion)
				return
			}

			// Initialize tabwriter
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0) // Use ' ' for padding, last param 0 for default
			fmt.Fprintln(w, "NLB NAME\tDNS\tIP(s)") // Header with tabs

			for _, nlb := range nlbs {
				ipsStr := strings.Join(nlb.IPs, ", ")
				if ipsStr == "" {
					ipsStr = "N/A"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", nlb.Name, nlb.DNSName, ipsStr)
			}

			w.Flush() // Flush the tabwriter buffer to stdout
		},
	}

	// Define flags for nlbCmd
	nlbCmd.Flags().StringVarP(&nlbRegion, "region", "r", "", "AWS region to fetch NLBs from (e.g., us-west-2)")
	if err := nlbCmd.MarkFlagRequired("region"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'region' as required: %v\n", err)
		os.Exit(1)
	}

	// Add subcommands to getCmd
	getCmd.AddCommand(nlbCmd)

	// Add getCmd to rootCmd
	rootCmd.AddCommand(getCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error executing command: %v\n", err)
		os.Exit(1)
	}
}
