// Package cli provides the command-line interface for the drift detector.
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/solomon-os/go-test/internal/aws"
	"github.com/solomon-os/go-test/internal/drift"
	"github.com/solomon-os/go-test/internal/models"
	"github.com/solomon-os/go-test/internal/reporter"
	"github.com/solomon-os/go-test/internal/terraform"
)

var (
	tfStatePath string
	region      string
	instanceIDs []string
	attributes  []string
	outputFmt   string
	timeout     time.Duration
)

var (
	setupOnce sync.Once
	rootCmd   = &cobra.Command{
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

func setup() {
	rootCmd.Flags().StringVarP(&tfStatePath, "tf-state", "t", "", "Path to Terraform state file (required)")
	rootCmd.Flags().StringVarP(&region, "region", "r", "us-east-1", "AWS region")
	rootCmd.Flags().StringSliceVarP(&instanceIDs, "instances", "i", nil, "Instance IDs to check (comma-separated, or checks all in state)")
	rootCmd.Flags().StringSliceVarP(&attributes, "attributes", "a", nil, "Attributes to check for drift (comma-separated)")
	rootCmd.Flags().StringVarP(&outputFmt, "output", "o", "text", "Output format: text, table, json")
	rootCmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Timeout for AWS API calls")
	must(rootCmd.MarkFlagRequired("tf-state"))

	rootCmd.AddCommand(detectCmd)
	detectCmd.Flags().StringVarP(&tfStatePath, "tf-state", "t", "", "Path to Terraform state file (required)")
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

// Run executes the CLI application.
func Run() error {
	setupOnce.Do(setup)
	return rootCmd.Execute()
}

func runDetector(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	tfParser := terraform.NewParser()
	tfInstances, err := tfParser.ParseFile(tfStatePath)
	if err != nil {
		return fmt.Errorf("failed to parse Terraform state: %w", err)
	}

	if len(tfInstances) == 0 {
		return fmt.Errorf("no EC2 instances found in Terraform state")
	}

	targetIDs := instanceIDs
	if len(targetIDs) == 0 {
		for id := range tfInstances {
			targetIDs = append(targetIDs, id)
		}
	}

	awsClient, err := aws.NewClient(ctx, region)
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	awsInstances, err := awsClient.GetInstances(ctx, targetIDs)
	if err != nil {
		return fmt.Errorf("failed to fetch AWS instances: %w", err)
	}

	awsInstanceMap := make(map[string]*models.EC2Instance)
	for _, inst := range awsInstances {
		awsInstanceMap[inst.InstanceID] = inst
	}

	detector := drift.NewDetector(attributes)
	report := detector.DetectMultiple(ctx, awsInstanceMap, tfInstances)

	rep := reporter.New(os.Stdout, reporter.Format(outputFmt))
	return rep.Report(report)
}

func runSingleDetect(cmd *cobra.Command, args []string) error {
	instanceID := args[0]
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	tfParser := terraform.NewParser()
	tfInstances, err := tfParser.ParseFile(tfStatePath)
	if err != nil {
		return fmt.Errorf("failed to parse Terraform state: %w", err)
	}

	tfInstance, err := tfParser.GetInstanceByID(tfInstances, instanceID)
	if err != nil {
		return err
	}

	awsClient, err := aws.NewClient(ctx, region)
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	awsInstance, err := awsClient.GetInstance(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("failed to fetch AWS instance: %w", err)
	}

	detector := drift.NewDetector(attributes)
	result := detector.Detect(awsInstance, tfInstance)

	rep := reporter.New(os.Stdout, reporter.Format(outputFmt))
	return rep.ReportSingle(result)
}

func runListAttributes(cmd *cobra.Command, args []string) {
	_, _ = fmt.Fprintln(os.Stdout, "Available attributes for drift detection:")
	_, _ = fmt.Fprintln(os.Stdout, strings.Repeat("-", 40))
	for _, attr := range drift.DefaultAttributes {
		_, _ = fmt.Fprintf(os.Stdout, "  - %s\n", attr)
	}
	_, _ = fmt.Fprintln(os.Stdout, "\nUse --attributes or -a flag to specify which attributes to check.")
	_, _ = fmt.Fprintln(os.Stdout, "If not specified, all default attributes will be checked.")
}
