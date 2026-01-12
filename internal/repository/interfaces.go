// Package repository defines data access interfaces following the repository pattern.
//
// The repository pattern provides an abstraction layer between the business logic
// and data access layers. This separation enables:
//   - Easier testing through mock implementations
//   - Flexibility to change data sources without modifying business logic
//   - Clear separation of concerns
//
// This package defines the contracts for accessing EC2 instance data from
// both AWS (actual state) and Terraform (desired state) sources.
//
// Example usage:
//
//	// Using the AWS repository
//	repo, _ := awsrepo.NewEC2Repository(ctx, "us-east-1")
//	instance, err := repo.GetByID(ctx, "i-1234567890abcdef0")
//
//	// Using the Terraform repository
//	tfRepo := tfrepo.NewRepository(parser, "terraform.tfstate")
//	instances, err := tfRepo.GetAll(ctx)
package repository

import (
	"context"

	"github.com/solomon-os/go-test/internal/errors"
	"github.com/solomon-os/go-test/internal/models"
)

// EC2Repository defines operations for accessing EC2 instance data.
// Implementations may fetch from AWS, local cache, or test fixtures.
type EC2Repository interface {
	// GetByID retrieves a single EC2 instance by its ID.
	// Returns ErrNotFound if the instance doesn't exist.
	GetByID(ctx context.Context, instanceID string) (*models.EC2Instance, error)

	// GetByIDs retrieves multiple EC2 instances by their IDs.
	// Missing instances are omitted from the result (no error for missing instances).
	GetByIDs(ctx context.Context, instanceIDs []string) ([]*models.EC2Instance, error)

	// List retrieves all EC2 instances matching the given filters.
	// If no filters are provided, all accessible instances are returned.
	List(ctx context.Context, filters ...Filter) ([]*models.EC2Instance, error)
}

// TerraformRepository defines operations for accessing Terraform state data.
// This abstracts the underlying storage (file, remote state, etc.).
type TerraformRepository interface {
	// GetByID retrieves a single instance from Terraform state.
	// Returns ErrNotFound if the instance doesn't exist in the state.
	GetByID(ctx context.Context, instanceID string) (*models.EC2Instance, error)

	// GetAll retrieves all EC2 instances from Terraform state.
	// Returns an empty map if no instances are found.
	GetAll(ctx context.Context) (map[string]*models.EC2Instance, error)

	// Refresh reloads the Terraform state from source.
	// This is useful when the state file may have changed.
	Refresh(ctx context.Context) error

	// FilePath returns the path to the Terraform state file.
	FilePath() string
}

// Filter represents a query filter for listing resources.
// Filters can be combined to narrow down results.
type Filter struct {
	// Name is the filter field name (e.g., "tag:Name", "instance-type").
	Name string
	// Values are the values to match against.
	Values []string
}

// NewFilter creates a new filter with the given name and values.
func NewFilter(name string, values ...string) Filter {
	return Filter{
		Name:   name,
		Values: values,
	}
}

// Common filters for EC2 instances.
var (
	// FilterRunning filters for running instances.
	FilterRunning = NewFilter("instance-state-name", "running")

	// FilterStopped filters for stopped instances.
	FilterStopped = NewFilter("instance-state-name", "stopped")
)

// TagFilter creates a filter for a specific tag key-value pair.
func TagFilter(key, value string) Filter {
	return NewFilter("tag:"+key, value)
}

// InstanceTypeFilter creates a filter for a specific instance type.
func InstanceTypeFilter(instanceType string) Filter {
	return NewFilter("instance-type", instanceType)
}

// Sentinel errors for repository operations.
var (
	// ErrNotFound indicates the requested resource was not found.
	ErrNotFound = errors.New(errors.CategoryInternal, "resource not found")

	// ErrNotLoaded indicates the repository data has not been loaded yet.
	ErrNotLoaded = errors.New(errors.CategoryInternal, "repository data not loaded")

	// ErrInvalidID indicates an invalid resource ID was provided.
	ErrInvalidID = errors.New(errors.CategoryInternal, "invalid resource ID")
)
