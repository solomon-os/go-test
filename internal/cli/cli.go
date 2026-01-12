// Package cli provides the command-line interface for the drift detector.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/solomon-os/go-test/internal/aws"
	"github.com/solomon-os/go-test/internal/drift"
	"github.com/solomon-os/go-test/internal/logger"
	"github.com/solomon-os/go-test/internal/models"
	"github.com/solomon-os/go-test/internal/reporter"
	"github.com/solomon-os/go-test/internal/terraform"
)

// AWSClient defines the interface for AWS EC2 operations.
type AWSClient interface {
	GetInstance(ctx context.Context, instanceID string) (*models.EC2Instance, error)
	GetInstances(ctx context.Context, instanceIDs []string) ([]*models.EC2Instance, error)
}

// App holds the CLI application dependencies.
type App struct {
	Parser       terraform.StateParser
	Detector     drift.Detector
	Reporter     reporter.DriftReporter
	AWSClient    AWSClient
	Output       io.Writer
	NewAWSClient func(ctx context.Context, region string) (AWSClient, error)
}

var (
	tfStatePath string
	region      string
	instanceIDs []string
	attributes  []string
	outputFmt   string
	timeout     time.Duration
	concurrency int
)

var (
	setupOnce  sync.Once
	defaultApp *App
	rootCmd    = &cobra.Command{
		Use:   "drift-detector",
		Short: "Detect infrastructure drift between AWS EC2 instances and Terraform",
		Long: `A tool to detect configuration drift between AWS EC2 instances
and their Terraform state/configuration files.

It compares the actual EC2 instance configuration in AWS with the expected
configuration defined in Terraform and reports any differences.`,
		RunE: runDetector,
	}

	detectCmd = &cobra.Command{
		Use:   "detect [instance-id]",
		Short: "Detect drift for a single instance",
		Args:  cobra.ExactArgs(1),
		RunE:  runSingleDetect,
	}

	listAttrsCmd = &cobra.Command{
		Use:   "list-attributes",
		Short: "List available attributes for drift detection",
		Run:   runListAttributes,
	}
)

func newDefaultApp() *App {
	return &App{
		Output: os.Stdout,
		NewAWSClient: func(ctx context.Context, region string) (AWSClient, error) {
			return aws.NewClient(ctx, region)
		},
	}
}

func setup() {
	defaultApp = newDefaultApp()

	rootCmd.Flags().
		StringVarP(&tfStatePath, "tf-state", "t", "", "Path to Terraform state file (required)")
	rootCmd.Flags().StringVarP(&region, "region", "r", "us-east-1", "AWS region")
	rootCmd.Flags().
		StringSliceVarP(&instanceIDs, "instances", "i", nil, "Instance IDs to check (comma-separated, or checks all in state)")
	rootCmd.Flags().
		StringSliceVarP(&attributes, "attributes", "a", nil, "Attributes to check for drift (comma-separated)")
	rootCmd.Flags().
		StringVarP(&outputFmt, "output", "o", "text", "Output format: text, table, json")
	rootCmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Timeout for AWS API calls")
	rootCmd.Flags().IntVar(&concurrency, "concurrency", drift.DefaultConcurrency, "Maximum concurrent drift checks")
	must(rootCmd.MarkFlagRequired("tf-state"))

	rootCmd.AddCommand(detectCmd)
	detectCmd.Flags().
		StringVarP(&tfStatePath, "tf-state", "t", "", "Path to Terraform state file (required)")
	detectCmd.Flags().StringVarP(&region, "region", "r", "us-east-1", "AWS region")
	detectCmd.Flags().StringSliceVarP(&attributes, "attributes", "a", nil, "Attributes to check")
	detectCmd.Flags().StringVarP(&outputFmt, "output", "o", "text", "Output format")
	must(detectCmd.MarkFlagRequired("tf-state"))

	rootCmd.AddCommand(listAttrsCmd)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func Run() error {
	setupOnce.Do(setup)
	logger.Debug("starting drift-detector CLI")
	return rootCmd.Execute()
}

func runDetector(cmd *cobra.Command, args []string) error {
	logger.Info("running drift detection", "tf_state", tfStatePath, "region", region)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	parser := getParser()
	tfInstances, err := parser.ParseFile(tfStatePath)
	if err != nil {
		logger.Error("failed to parse Terraform state", "path", tfStatePath, "error", err)
		return fmt.Errorf("failed to parse Terraform state: %w", err)
	}

	if len(tfInstances) == 0 {
		logger.Error("no EC2 instances found in Terraform state", "path", tfStatePath)
		return fmt.Errorf("no EC2 instances found in Terraform state")
	}

	targetIDs := instanceIDs
	if len(targetIDs) == 0 {
		for id := range tfInstances {
			targetIDs = append(targetIDs, id)
		}
	}
	logger.Debug("target instances", "count", len(targetIDs))

	awsClient, err := getAWSClient(ctx, region)
	if err != nil {
		logger.Error("failed to create AWS client", "region", region, "error", err)
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	awsInstances, err := awsClient.GetInstances(ctx, targetIDs)
	if err != nil {
		logger.Error("failed to fetch AWS instances", "error", err)
		return fmt.Errorf("failed to fetch AWS instances: %w", err)
	}

	awsInstanceMap := make(map[string]*models.EC2Instance)
	for _, inst := range awsInstances {
		awsInstanceMap[inst.InstanceID] = inst
	}

	detector := getDetector()
	report := detector.DetectMultiple(ctx, awsInstanceMap, tfInstances)

	logger.Info(
		"drift detection completed",
		"total",
		report.TotalInstances,
		"drifted",
		report.DriftedInstances,
	)
	rep := getReporter()
	return rep.Report(report)
}

func runSingleDetect(cmd *cobra.Command, args []string) error {
	instanceID := args[0]
	logger.Info(
		"detecting drift for single instance",
		"instance_id",
		instanceID,
		"tf_state",
		tfStatePath,
	)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	parser := getParser()
	tfInstances, err := parser.ParseFile(tfStatePath)
	if err != nil {
		logger.Error("failed to parse Terraform state", "path", tfStatePath, "error", err)
		return fmt.Errorf("failed to parse Terraform state: %w", err)
	}

	tfInstance, err := parser.GetInstanceByID(tfInstances, instanceID)
	if err != nil {
		logger.Error(
			"instance not found in Terraform state",
			"instance_id",
			instanceID,
			"error",
			err,
		)
		return err
	}

	awsClient, err := getAWSClient(ctx, region)
	if err != nil {
		logger.Error("failed to create AWS client", "region", region, "error", err)
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	awsInstance, err := awsClient.GetInstance(ctx, instanceID)
	if err != nil {
		logger.Error("failed to fetch AWS instance", "instance_id", instanceID, "error", err)
		return fmt.Errorf("failed to fetch AWS instance: %w", err)
	}

	detector := getDetector()
	result := detector.Detect(awsInstance, tfInstance)

	logger.Info(
		"single instance drift detection completed",
		"instance_id",
		instanceID,
		"has_drift",
		result.HasDrift,
	)
	rep := getReporter()
	return rep.ReportSingle(result)
}

func runListAttributes(cmd *cobra.Command, args []string) {
	out := defaultApp.Output
	writef(out, "Available attributes for drift detection:\n")
	writef(out, "%s\n", strings.Repeat("-", 40))
	for _, attr := range drift.DefaultAttributes {
		writef(out, "  - %s\n", attr)
	}
	writef(out, "\nUse --attributes or -a flag to specify which attributes to check.\n")
	writef(out, "If not specified, all default attributes will be checked.\n")
}

func writef(w io.Writer, format string, args ...any) {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		logger.Warn("failed to write output", "error", err)
	}
}

func getParser() terraform.StateParser {
	if defaultApp.Parser != nil {
		return defaultApp.Parser
	}
	return terraform.NewParser()
}

func getDetector() drift.Detector {
	if defaultApp.Detector != nil {
		return defaultApp.Detector
	}
	return drift.NewDetector(attributes, drift.WithConcurrency(concurrency))
}

func getReporter() reporter.DriftReporter {
	if defaultApp.Reporter != nil {
		return defaultApp.Reporter
	}
	return reporter.New(defaultApp.Output, reporter.Format(outputFmt))
}

func getAWSClient(ctx context.Context, region string) (AWSClient, error) {
	if defaultApp.AWSClient != nil {
		return defaultApp.AWSClient, nil
	}
	return defaultApp.NewAWSClient(ctx, region)
}
