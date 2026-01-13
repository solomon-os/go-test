// Package factory provides factories for creating configured application components.
//
// This package implements the Factory pattern to centralize component creation
// and configuration. It enables:
//   - Consistent component initialization across the application
//   - Easy configuration management
//   - Dependency injection for testability
//   - Clear separation between configuration and usage
//
// Example usage:
//
//	config := factory.DefaultConfig()
//	config.AWSRegion = "us-west-2"
//	config.Concurrency = 20
//
//	f := factory.New(config)
//	service, err := f.CreateDriftService(ctx)
package factory

import (
	"context"
	"fmt"
	"io"

	"github.com/solomon-os/go-test/internal/aws"
	"github.com/solomon-os/go-test/internal/drift"
	"github.com/solomon-os/go-test/internal/models"
	"github.com/solomon-os/go-test/internal/reporter"
	"github.com/solomon-os/go-test/internal/reporter/formatter"
	"github.com/solomon-os/go-test/internal/repository"
	awsrepo "github.com/solomon-os/go-test/internal/repository/aws"
	tfrepo "github.com/solomon-os/go-test/internal/repository/terraform"
	"github.com/solomon-os/go-test/internal/retry"
	"github.com/solomon-os/go-test/internal/terraform"
)

// Config holds all configuration for the application.
type Config struct {
	// AWSRegion is the AWS region to use for EC2 API calls.
	AWSRegion string

	// TerraformPath is the path to the Terraform state file.
	TerraformPath string

	// Attributes is the list of attributes to check for drift.
	// If empty, default attributes are used.
	Attributes []string

	// OutputFormat is the format for output reports (text, json, table).
	OutputFormat string

	// Concurrency is the maximum number of concurrent drift checks.
	Concurrency int

	// RetryConfig configures retry behavior for AWS API calls.
	RetryConfig retry.Config
}

// DefaultConfig returns configuration with sensible defaults.
func DefaultConfig() Config {
	return Config{
		AWSRegion:    "us-east-1",
		Attributes:   nil, // Use default attributes
		OutputFormat: "text",
		Concurrency:  drift.DefaultConcurrency,
		RetryConfig:  retry.AWSConfig,
	}
}

// Factory creates application components with configured dependencies.
type Factory struct {
	config Config

	// Cached components for reuse
	awsClient  *aws.Client
	parser     terraform.StateParser
	formatters *formatter.Registry
}

// New creates a new Factory with the given configuration.
func New(config Config) *Factory {
	return &Factory{
		config:     config,
		formatters: formatter.NewRegistry(),
	}
}

// Config returns the factory's configuration.
func (f *Factory) Config() Config {
	return f.config
}

// CreateAWSClient creates a configured AWS client.
// The client is cached and reused for subsequent calls.
func (f *Factory) CreateAWSClient(ctx context.Context) (*aws.Client, error) {
	if f.awsClient != nil {
		return f.awsClient, nil
	}

	client, err := aws.NewClient(ctx, f.config.AWSRegion,
		aws.WithRetryConfig(f.config.RetryConfig))
	if err != nil {
		return nil, err
	}

	f.awsClient = client
	return client, nil
}

// CreateParser creates a Terraform parser.
// The parser is cached and reused for subsequent calls.
func (f *Factory) CreateParser() terraform.StateParser {
	if f.parser != nil {
		return f.parser
	}

	f.parser = terraform.NewParser()
	return f.parser
}

// CreateEC2Repository creates an EC2 repository.
func (f *Factory) CreateEC2Repository(ctx context.Context) (repository.EC2Repository, error) {
	client, err := f.CreateAWSClient(ctx)
	if err != nil {
		return nil, err
	}
	return awsrepo.NewEC2Repository(client), nil
}

// CreateTerraformRepository creates a Terraform repository.
func (f *Factory) CreateTerraformRepository() repository.TerraformRepository {
	parser := f.CreateParser()
	return tfrepo.NewRepository(parser, f.config.TerraformPath)
}

// CreateDetector creates a configured drift detector.
func (f *Factory) CreateDetector() drift.Detector {
	return drift.NewDetector(f.config.Attributes,
		drift.WithConcurrency(f.config.Concurrency))
}

// CreateReporter creates a configured reporter.
func (f *Factory) CreateReporter(w io.Writer) reporter.DriftReporter {
	return reporter.New(w, reporter.Format(f.config.OutputFormat))
}

// CreateFormatter returns a formatter for the configured output format.
func (f *Factory) CreateFormatter() (formatter.Formatter, bool) {
	return f.formatters.Get(f.config.OutputFormat)
}

// FormattersRegistry returns the formatters registry for custom formatter registration.
func (f *Factory) FormattersRegistry() *formatter.Registry {
	return f.formatters
}

// DriftService orchestrates drift detection using repositories and detector.
// It provides a high-level API for performing drift detection operations.
type DriftService struct {
	awsRepo  repository.EC2Repository
	tfRepo   repository.TerraformRepository
	detector drift.Detector
}

// NewDriftService creates a new DriftService with the given dependencies.
func NewDriftService(
	awsRepo repository.EC2Repository,
	tfRepo repository.TerraformRepository,
	detector drift.Detector,
) *DriftService {
	return &DriftService{
		awsRepo:  awsRepo,
		tfRepo:   tfRepo,
		detector: detector,
	}
}

// CreateDriftService creates a fully configured drift service.
func (f *Factory) CreateDriftService(ctx context.Context) (*DriftService, error) {
	awsRepo, err := f.CreateEC2Repository(ctx)
	if err != nil {
		return nil, err
	}

	tfRepo := f.CreateTerraformRepository()
	detector := f.CreateDetector()

	return NewDriftService(awsRepo, tfRepo, detector), nil
}

// DetectDrift performs drift detection for the specified instances.
// If instanceIDs is empty, all instances from the Terraform state are checked.
func (s *DriftService) DetectDrift(
	ctx context.Context,
	instanceIDs []string,
) (*models.DriftReport, error) {
	// Load Terraform state
	tfInstances, err := s.tfRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load Terraform state: %w", err)
	}

	// Determine which instances to check
	if len(instanceIDs) == 0 {
		instanceIDs = make([]string, 0, len(tfInstances))
		for id := range tfInstances {
			instanceIDs = append(instanceIDs, id)
		}
	}

	// Fetch AWS instances
	awsInstances, err := s.awsRepo.GetByIDs(ctx, instanceIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch AWS instances: %w", err)
	}

	// Convert to map for detector
	awsMap := make(map[string]*models.EC2Instance)
	for _, inst := range awsInstances {
		awsMap[inst.InstanceID] = inst
	}

	// Perform drift detection
	return s.detector.DetectMultiple(ctx, awsMap, tfInstances), nil
}

// AWSDrifter returns the EC2 repository.
func (s *DriftService) AWSRepo() repository.EC2Repository {
	return s.awsRepo
}

// TerraformRepo returns the Terraform repository.
func (s *DriftService) TerraformRepo() repository.TerraformRepository {
	return s.tfRepo
}

// Detector returns the drift detector.
func (s *DriftService) Detector() drift.Detector {
	return s.detector
}
