package aws

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ASGData holds information about an Auto Scaling Group
type ASGData struct {
	Name           string
	Status         string
	MinSize        int64
	MaxSize        int64
	DesiredSize    int64
	LaunchTemplate string
	Instances      []InstanceData
	Activities     []ActivityData
	CPUUtilization int // For demo or would be fetched from CloudWatch
	NetworkUsage   int // For demo or would be fetched from CloudWatch
	ScalingStatus  string
}

// InstanceData holds information about an EC2 instance in the ASG
type InstanceData struct {
	ID             string
	State          string
	Health         string
	IP             string
	Type           string
	LaunchTime     time.Time
	ProtectedScale bool
}

// ActivityData holds information about ASG activities
type ActivityData struct {
	Time        time.Time
	Type        string
	InstanceID  string
	Status      string
	Description string
}

// MonitorOptions contains options for the ASG monitor
type MonitorOptions struct {
	RefreshInterval int
	Region          string
	Profile         string
}

// Monitor starts a terminal-based monitor for an AWS Auto Scaling Group
func Monitor(asgName string, options MonitorOptions) error {
	// Create a new application
	app := tview.NewApplication()

	// Create a flex container that will hold our UI components
	flex := tview.NewFlex().SetDirection(tview.FlexRow)

	// Initialize AWS session
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

	if options.Region != "" {
		sess.Config.Region = aws.String(options.Region)
	}

	// Get initial ASG data
	asgData, err := fetchASGData(sess, asgName)
	if err != nil {
		return fmt.Errorf("failed to fetch ASG data: %v", err)
	}

	// Create our main text view
	dashboard := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true)

	// Log view at the bottom
	logView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetTextColor(tcell.ColorLightGray)

	// Add components to the flex container
	flex.AddItem(dashboard, 0, 1, false)
	flex.AddItem(logView, 7, 1, false)

	// Function to update the dashboard display
	updateDashboard := func() {
		dashboard.Clear()
		renderASGDashboard(dashboard, asgData)

		// Update the log with recent activity
		logView.Clear()
		fmt.Fprintf(logView, "[yellow]LIVE LOG:[white]\n")
		fmt.Fprintf(logView, "[gray]%s[white] Monitoring ASG '%s'...\n", time.Now().Format("[15:04:05]"), asgData.Name)

		// Add the most recent activities to the log
		for i := 0; i < len(asgData.Activities) && i < 5; i++ {
			activity := asgData.Activities[i]
			fmt.Fprintf(logView, "[gray]%s[white] %s\n", activity.Time.Format("[15:04:05]"), activity.Description)
		}
	}

	// Set up a function to handle keyboard input
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			app.Stop()
		} else if event.Rune() == 'r' {
			// Refresh data
			newData, err := fetchASGData(sess, asgName)
			if err == nil {
				asgData = newData
				updateDashboard()
			} else {
				fmt.Fprintf(logView, "[red]%s[white] Error refreshing data: %v\n", time.Now().Format("[15:04:05]"), err)
			}
		}
		return event
	})

	// Initial render
	updateDashboard()

	// Set up a ticker to update the display periodically
	refreshInterval := time.Duration(options.RefreshInterval) * time.Second
	if refreshInterval == 0 {
		refreshInterval = 5 * time.Second // Default to 5 seconds
	}

	go func() {
		ticker := time.NewTicker(refreshInterval)
		for {
			select {
			case <-ticker.C:
				app.QueueUpdateDraw(func() {
					newData, err := fetchASGData(sess, asgName)
					if err == nil {
						asgData = newData
						updateDashboard()
					} else {
						fmt.Fprintf(logView, "[red]%s[white] Error refreshing data: %v\n", time.Now().Format("[15:04:05]"), err)
					}
				})
			}
		}
	}()

	// Set the flex container as the root of the application and start
	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		return fmt.Errorf("error running application: %v", err)
	}

	return nil
}

// renderASGDashboard creates a formatted display of ASG information
func renderASGDashboard(view *tview.TextView, asg ASGData) {
	// Header
	fmt.Fprintf(view, "╔═══ r-refresh ═════════ AWS Auto Scaling Group Monitor ══════ q-quit ===═══════╗\n")
	fmt.Fprintf(view, "║ ASG Name: %-56s Refreshed: %s ║\n", asg.Name, time.Now().Format("15:04:05"))
	fmt.Fprintf(view, "╠═══════════════════════════════════════════════════════════════════════════════╣\n")

	// ASG Status
	fmt.Fprintf(view, "║ Status: %-67s ║\n", asg.Status)

	// Capacity bar
	capacityBar := createProgressBar(int(asg.DesiredSize), int(asg.MaxSize), 10)
	fmt.Fprintf(view, "║ Capacity: [%s] %d/%d  (Min: %d, Desired: %d, Max: %d)%s ║\n",
		capacityBar,
		asg.DesiredSize,
		asg.MaxSize,
		asg.MinSize,
		asg.DesiredSize,
		asg.MaxSize,
		strings.Repeat(" ", 20))

	fmt.Fprintf(view, "║ Launch Template: %-56s ║\n", asg.LaunchTemplate)

	// Instances section
	fmt.Fprintf(view, "╠═════════════════════════════ INSTANCES ══════════════════════════════════════╣\n")
	fmt.Fprintf(view, "║ ID                    │ STATE     │ HEALTH   │ IP        │ TYPE     │ AGE     ║\n")
	fmt.Fprintf(view, "╟──────────────────────┼──────────┼─────────┼──────────┼─────────┼─────────╢\n")

	for _, instance := range asg.Instances {
		ageDuration := time.Since(instance.LaunchTime)
		ageStr := fmt.Sprintf("%dh %dm", int(ageDuration.Hours()), int(ageDuration.Minutes())%60)

		fmt.Fprintf(view, "║ %-20s │ %-8s │ %-7s │ %-8s │ %-7s │ %-7s ║\n",
			instance.ID,
			instance.State,
			instance.Health,
			instance.IP,
			instance.Type,
			ageStr)
	}

	// Activities section
	fmt.Fprintf(view, "╠═════════════════════════════ ACTIVITIES ══════════════════════════════════════╣\n")
	fmt.Fprintf(view, "║ TIME     │ TYPE         │ INSTANCE           │ STATUS    │ DETAILS           ║\n")
	fmt.Fprintf(view, "╟─────────┼─────────────┼───────────────────┼──────────┼──────────────────────╢\n")

	for _, activity := range asg.Activities {
		fmt.Fprintf(view, "║ %-7s │ %-11s │ %-17s │ %-8s │ %-18s ║\n",
			activity.Time.Format("15:04:05"),
			activity.Type,
			activity.InstanceID,
			activity.Status,
			truncateString(activity.Description, 18))
	}

	// Metrics section
	fmt.Fprintf(view, "╠═════════════════════════════ METRICS ═════════════════════════════════════════╣\n")

	// CPU usage bar
	cpuBar := createProgressBar(asg.CPUUtilization, 100, 10)
	networkBar := createProgressBar(asg.NetworkUsage, 100, 10)

	fmt.Fprintf(view, "║ CPU: %d%% [%s] │ Network: 256MB/s [%s] │ Scaling: %-10s ║\n",
		asg.CPUUtilization,
		cpuBar,
		networkBar,
		asg.ScalingStatus)

	// Footer
	fmt.Fprintf(view, "╚═══════════════════════════════════════════════════════════════════════════════╝\n")
}

// createProgressBar creates a text-based progress bar
func createProgressBar(current, max, width int) string {
	filledWidth := int(float64(current) / float64(max) * float64(width))
	if filledWidth > width {
		filledWidth = width
	}

	bar := strings.Repeat("•", filledWidth) + strings.Repeat("○", width-filledWidth)
	return bar
}

// fetchASGData gets ASG information from AWS
func fetchASGData(sess *session.Session, asgName string) (ASGData, error) {
	// Create AutoScaling service client
	svc := autoscaling.New(sess)

	// Get ASG information
	asgInput := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{aws.String(asgName)},
	}

	asgOutput, err := svc.DescribeAutoScalingGroups(asgInput)
	if err != nil {
		return ASGData{}, err
	}

	// Check if ASG exists
	if len(asgOutput.AutoScalingGroups) == 0 {
		return ASGData{}, fmt.Errorf("ASG not found: %s", asgName)
	}

	asg := asgOutput.AutoScalingGroups[0]

	// Create ASGData object
	asgData := ASGData{
		Name:        *asg.AutoScalingGroupName,
		Status:      "ACTIVE", // ASG doesn't have a direct status field
		MinSize:     *asg.MinSize,
		MaxSize:     *asg.MaxSize,
		DesiredSize: *asg.DesiredCapacity,
	}

	// Set launch template info if available
	if asg.LaunchTemplate != nil {
		ltName := *asg.LaunchTemplate.LaunchTemplateName
		ltVersion := *asg.LaunchTemplate.Version
		asgData.LaunchTemplate = fmt.Sprintf("%s (v%s)", ltName, ltVersion)
	} else if asg.MixedInstancesPolicy != nil && asg.MixedInstancesPolicy.LaunchTemplate != nil {
		ltName := *asg.MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification.LaunchTemplateName
		ltVersion := *asg.MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification.Version
		asgData.LaunchTemplate = fmt.Sprintf("%s (v%s) [Mixed]", ltName, ltVersion)
	} else if asg.LaunchConfigurationName != nil {
		asgData.LaunchTemplate = fmt.Sprintf("LC: %s", *asg.LaunchConfigurationName)
	} else {
		asgData.LaunchTemplate = "No template/config"
	}

	// Get instance information
	ec2svc := ec2.New(sess)

	for _, instance := range asg.Instances {
		ipAddr, ipErr := GetInstancePrivateIP(sess, *instance.InstanceId) // Call and get both return values
		if ipErr != nil {
			// Log the error or handle it appropriately
			fmt.Printf("Warning: could not get IP for instance %s: %v\n", *instance.InstanceId, ipErr)
			ipAddr = "N/A" // Set a placeholder value if IP couldn't be retrieved
		}
		instanceData := InstanceData{
			ID:             *instance.InstanceId,
			State:          *instance.LifecycleState,
			Health:         *instance.HealthStatus,
			IP:             ipAddr,
			ProtectedScale: *instance.ProtectedFromScaleIn,
		}

		// Get instance type and launch time from EC2 API
		ec2Input := &ec2.DescribeInstancesInput{
			InstanceIds: []*string{instance.InstanceId},
		}

		ec2Output, err := ec2svc.DescribeInstances(ec2Input)
		if err == nil && len(ec2Output.Reservations) > 0 && len(ec2Output.Reservations[0].Instances) > 0 {
			ec2Instance := ec2Output.Reservations[0].Instances[0]
			instanceData.Type = *ec2Instance.InstanceType
			instanceData.LaunchTime = *ec2Instance.LaunchTime
		} else {
			// Default launch time if we can't get it
			instanceData.Type = "unknown"
			instanceData.LaunchTime = time.Now()
		}

		asgData.Instances = append(asgData.Instances, instanceData)
	}

	// Get scaling activities
	activityInput := &autoscaling.DescribeScalingActivitiesInput{
		AutoScalingGroupName: aws.String(asgName),
		MaxRecords:           aws.Int64(10),
	}

	activityOutput, err := svc.DescribeScalingActivities(activityInput)
	if err == nil {
		for _, activity := range activityOutput.Activities {
			activityType := "Group Update"
			instanceID := "-"
			description := *activity.Description

			// Parse activity type and instance ID from description
			if strings.Contains(description, "Launching") {
				activityType = "Launch"
				parts := strings.Split(description, ":")
				if len(parts) > 1 {
					instanceID = strings.TrimSpace(parts[1])
				}
			} else if strings.Contains(description, "Terminating") {
				activityType = "Terminate"
				parts := strings.Split(description, ":")
				if len(parts) > 1 {
					instanceID = strings.TrimSpace(parts[1])
				}
			}

			activityData := ActivityData{
				Time:        *activity.StartTime,
				Type:        activityType,
				InstanceID:  instanceID,
				Status:      *activity.StatusCode,
				Description: truncateString(extractCauseInfo(*activity.Cause), 60),
			}

			asgData.Activities = append(asgData.Activities, activityData)
		}
	}

	// For demo purposes, we'll set some mock values for CPU and network
	// In a real app, you would get these from CloudWatch
	asgData.CPUUtilization = 72
	asgData.NetworkUsage = 75
	asgData.ScalingStatus = "ACTIVE"

	return asgData, nil
}

// Helper function to truncate strings
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength-3] + "..."
}

// Extract useful information from the cause message
func extractCauseInfo(cause string) string {
	if strings.Contains(cause, "user request") {
		return "User initiated"
	} else if strings.Contains(cause, "health-check") {
		return "Failed health check"
	} else if strings.Contains(cause, "capacity from") {
		parts := strings.Split(cause, "capacity from")
		if len(parts) > 1 {
			scaleParts := strings.Split(parts[1], "to")
			if len(scaleParts) > 1 {
				from := strings.TrimSpace(scaleParts[0])
				to := strings.TrimSpace(strings.Split(scaleParts[1], ".")[0])
				return fmt.Sprintf("Scaling %s→%s", from, to)
			}
		}
	}
	return "Scale activity"
}

// GetInstancePrivateIP retrieves the private IP address for a given EC2 instance ID.
// It requires an initialized AWS session.
func GetInstancePrivateIP(sess *session.Session, instanceID string) (string, error) {
	// Create an EC2 service client from the session
	ec2Svc := ec2.New(sess)

	// Prepare the input for DescribeInstances
	input := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(instanceID)},
		// Add filters if needed, e.g., to ensure the instance is running
		// Filters: []*ec2.Filter{
		// 	{
		// 		Name:   aws.String("instance-state-name"),
		// 		Values: []*string{aws.String("running")},
		// 	},
		// },
	}

	// Call DescribeInstances
	result, err := ec2Svc.DescribeInstances(input)
	if err != nil {
		return "", fmt.Errorf("failed to describe instance %s: %w", instanceID, err)
	}

	// Process the results
	// Check if any reservations were returned
	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return "", fmt.Errorf("instance not found: %s", instanceID)
	}

	// Get the first instance (should be the only one when querying by specific ID)
	instance := result.Reservations[0].Instances[0]

	// Check if the instance has a private IP address
	if instance.PrivateIpAddress == nil {
		return "", fmt.Errorf("instance %s does not have a private IP address", instanceID)
	}

	// Return the private IP address
	privateIP := aws.StringValue(instance.PrivateIpAddress)
	return privateIP, nil
}
