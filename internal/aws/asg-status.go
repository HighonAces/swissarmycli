package aws

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
)

// --- Data Structures are NOT redefined here ---
// They are defined in asg-status-stream.go within the same 'aws' package
// and are therefore accessible to this file.

// --- Implementation of OnlyStatus ---

func OnlyStatus(asgName string, options MonitorOptions) error { // Uses MonitorOptions struct from asg-status-stream.go
	// 1. Initialize AWS session
	var sess *session.Session
	var err error

	sessOptions := session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}

	if options.Profile != "" {
		sessOptions.Profile = options.Profile
	}

	sess, err = session.NewSessionWithOptions(sessOptions)
	if err != nil {
		return fmt.Errorf("failed to create AWS session: %v", err)
	}

	// Apply region if specified or use session's default
	if options.Region != "" {
		sess.Config.Region = aws.String(options.Region)
	} else if sess.Config.Region == nil || *sess.Config.Region == "" {
		fmt.Println("Warning: AWS region not specified via flag or default config. AWS calls might fail if region is required.")
	} else {
		fmt.Printf("Using AWS region from config/environment: %s\n", aws.StringValue(sess.Config.Region))
	}

	// 2. Fetch ASG data using the helper function below
	// Note: fetchASGData is defined in asg-status-stream.go in this scenario
	asgData, err := fetchASGData(sess, asgName) // Uses ASGData struct from asg-status-stream.go
	if err != nil {
		return fmt.Errorf("failed to fetch ASG data: %v", err)
	}

	// 3. Print the formatted status
	fmt.Println("--------------------------------------------------")
	fmt.Printf(" Auto Scaling Group Status: %s\n", asgData.Name)
	fmt.Println("--------------------------------------------------")

	fmt.Printf("  %-20s %s\n", "Status:", asgData.Status)
	fmt.Printf("  %-20s Min=%d, Max=%d, Desired=%d\n", "Capacity:", asgData.MinSize, asgData.MaxSize, asgData.DesiredSize)
	fmt.Printf("  %-20s %s\n", "Launch Template:", asgData.LaunchTemplate)

	fmt.Println("\n  Instances:")
	if len(asgData.Instances) == 0 {
		fmt.Println("    No instances found in the group.")
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0) // Align columns
		fmt.Fprintln(w, "    ID\tSTATE\tHEALTH\tAZ\tTYPE\tAGE\tPROTECTED")

		// Uses InstanceData struct from asg-status-stream.go
		for _, instance := range asgData.Instances {
			ageDuration := time.Since(instance.LaunchTime)
			var ageStr string // Concise age format
			if ageDuration.Hours() >= 24 {
				ageStr = fmt.Sprintf("%.1fd", ageDuration.Hours()/24.0)
			} else if ageDuration.Hours() >= 1 {
				ageStr = fmt.Sprintf("%.1fh", ageDuration.Hours())
			} else {
				ageStr = fmt.Sprintf("%.0fm", ageDuration.Minutes())
			}

			fmt.Fprintf(w, "    %s\t%s\t%s\t%s\t%s\t%s\t%t\n",
				instance.ID,
				instance.State,
				instance.Health,
				instance.AZ,
				instance.Type,
				ageStr,
				instance.ProtectedScale)
		}
		w.Flush() // Print the formatted table
	}

	// Recent Activities Summary
	fmt.Println("\n  Recent Activities (limit 5):")
	if len(asgData.Activities) == 0 {
		fmt.Println("    No recent activities found.")
	} else {
		limit := 5
		if len(asgData.Activities) < limit {
			limit = len(asgData.Activities)
		}
		// Uses ActivityData struct from asg-status-stream.go
		for i := 0; i < limit; i++ {
			activity := asgData.Activities[i]
			fmt.Printf("    - %s [%s]: %s (%s)\n",
				activity.Time.Format("2006-01-02 15:04:05 MST"), // Standard timestamp
				activity.Status,
				activity.Description, // Assumes Description is already summarized by fetchASGData
				activity.Type)        // Assumes Type is populated by fetchASGData
		}
	}

	fmt.Println("--------------------------------------------------")

	return nil // Success
}

// --- Helper Functions ---
// Note: If fetchASGData and its helpers (parseActivityType, extractCauseInfo, truncateString)
// are defined in asg-status-stream.go, they do NOT need to be redefined here.
// If they were intended to be only in this file, they should be moved here,
// and the struct definitions should also be moved here, and removed from asg-status-stream.go.
//
// Based on the request "Do not modify asg-status-stream at all", we assume
// fetchASGData and its helpers ARE defined in asg-status-stream.go and are accessible here.
