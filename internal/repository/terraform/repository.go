// Package terraform implements repository interfaces for Terraform state access.
//
// This package provides a TerraformRepository implementation that parses
// Terraform state files (both JSON and HCL formats) to extract EC2 instance
// configurations.
package terraform

import (
	"context"
	"sync"

	"github.com/solomon-os/go-test/internal/models"
	"github.com/solomon-os/go-test/internal/repository"
	tf "github.com/solomon-os/go-test/internal/terraform"
)

// Repository implements repository.TerraformRepository.
// It caches parsed instances and supports refresh operations.
type Repository struct {
	parser   tf.StateParser
	filePath string

	mu        sync.RWMutex
	instances map[string]*models.EC2Instance
	loaded    bool
}

// NewRepository creates a new Terraform repository.
func NewRepository(parser tf.StateParser, filePath string) *Repository {
	return &Repository{
		parser:   parser,
		filePath: filePath,
	}
}

// GetByID retrieves a single instance from Terraform state.
func (r *Repository) GetByID(ctx context.Context, instanceID string) (*models.EC2Instance, error) {
	if instanceID == "" {
		return nil, repository.ErrInvalidID
	}

	// Ensure data is loaded
	if err := r.ensureLoaded(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	inst, ok := r.instances[instanceID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return inst, nil
}

// GetAll retrieves all EC2 instances from Terraform state.
func (r *Repository) GetAll(ctx context.Context) (map[string]*models.EC2Instance, error) {
	// Ensure data is loaded
	if err := r.ensureLoaded(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to prevent external mutation
	result := make(map[string]*models.EC2Instance, len(r.instances))
	for k, v := range r.instances {
		result[k] = v
	}
	return result, nil
}

// Refresh reloads the Terraform state from source.
func (r *Repository) Refresh(ctx context.Context) error {
	instances, err := r.parser.ParseFile(r.filePath)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.instances = instances
	r.loaded = true
	r.mu.Unlock()

	return nil
}

// FilePath returns the path to the Terraform state file.
func (r *Repository) FilePath() string {
	return r.filePath
}

// IsLoaded returns whether the repository data has been loaded.
func (r *Repository) IsLoaded() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.loaded
}

// InstanceCount returns the number of instances in the repository.
func (r *Repository) InstanceCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.instances)
}

// ensureLoaded loads data if not already loaded.
func (r *Repository) ensureLoaded(ctx context.Context) error {
	r.mu.RLock()
	loaded := r.loaded
	r.mu.RUnlock()

	if loaded {
		return nil
	}

	return r.Refresh(ctx)
}

// Verify interface compliance at compile time.
var _ repository.TerraformRepository = (*Repository)(nil)
