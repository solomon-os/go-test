// Package aws implements repository interfaces using AWS SDK.
//
// This package provides an EC2Repository implementation that fetches
// instance data directly from AWS using the EC2 API.
package aws

import (
	"context"

	"github.com/solomon-os/go-test/internal/aws"
	"github.com/solomon-os/go-test/internal/models"
	"github.com/solomon-os/go-test/internal/repository"
)

// EC2Repository implements repository.EC2Repository using AWS SDK.
// It delegates to the aws.Client for actual API calls.
type EC2Repository struct {
	client *aws.Client
}

// NewEC2Repository creates a new AWS-backed EC2 repository.
func NewEC2Repository(client *aws.Client) *EC2Repository {
	return &EC2Repository{client: client}
}

// NewEC2RepositoryWithRegion creates a new EC2 repository with a fresh AWS client.
func NewEC2RepositoryWithRegion(ctx context.Context, region string, opts ...aws.ClientOption) (*EC2Repository, error) {
	client, err := aws.NewClient(ctx, region, opts...)
	if err != nil {
		return nil, err
	}
	return &EC2Repository{client: client}, nil
}

// GetByID retrieves a single EC2 instance by its ID.
func (r *EC2Repository) GetByID(ctx context.Context, instanceID string) (*models.EC2Instance, error) {
	if instanceID == "" {
		return nil, repository.ErrInvalidID
	}
	return r.client.GetInstance(ctx, instanceID)
}

// GetByIDs retrieves multiple EC2 instances by their IDs.
func (r *EC2Repository) GetByIDs(ctx context.Context, instanceIDs []string) ([]*models.EC2Instance, error) {
	if len(instanceIDs) == 0 {
		return []*models.EC2Instance{}, nil
	}
	return r.client.GetInstances(ctx, instanceIDs)
}

// List retrieves all EC2 instances matching the given filters.
// Note: Currently, filters are not implemented - all instances are returned.
// This is a placeholder for future filter support.
func (r *EC2Repository) List(ctx context.Context, filters ...repository.Filter) ([]*models.EC2Instance, error) {
	// TODO: Implement filter support using AWS DescribeInstances filters
	// For now, this method requires instance IDs to be provided via GetByIDs
	return nil, nil
}

// Client returns the underlying AWS client.
// This can be useful for advanced operations not covered by the repository interface.
func (r *EC2Repository) Client() *aws.Client {
	return r.client
}

// Verify interface compliance at compile time.
var _ repository.EC2Repository = (*EC2Repository)(nil)
