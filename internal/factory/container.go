// Package factory provides factories for creating configured application components.
package factory

import (
	"context"

	"github.com/solomon-os/go-test/internal/drift"
	"github.com/solomon-os/go-test/internal/reporter"
	"github.com/solomon-os/go-test/internal/repository"
)

// Container holds application dependencies for dependency injection.
// It provides a central place to access all components and is useful
// for testing when you need to inject mock implementations.
type Container struct {
	EC2Repository repository.EC2Repository
	TFRepository  repository.TerraformRepository
	Detector      drift.Detector
	Reporter      reporter.DriftReporter
}

// Builder creates containers with optional overrides for testing.
// Use the builder pattern to construct containers with specific
// mock implementations.
type Builder struct {
	factory   *Factory
	overrides map[string]any
}

// NewBuilder creates a new container builder.
func NewBuilder(factory *Factory) *Builder {
	return &Builder{
		factory:   factory,
		overrides: make(map[string]any),
	}
}

// WithEC2Repository sets a custom EC2 repository.
func (b *Builder) WithEC2Repository(r repository.EC2Repository) *Builder {
	b.overrides["ec2repo"] = r
	return b
}

// WithTerraformRepository sets a custom Terraform repository.
func (b *Builder) WithTerraformRepository(r repository.TerraformRepository) *Builder {
	b.overrides["tfrepo"] = r
	return b
}

// WithDetector sets a custom detector.
func (b *Builder) WithDetector(d drift.Detector) *Builder {
	b.overrides["detector"] = d
	return b
}

// WithReporter sets a custom reporter.
func (b *Builder) WithReporter(r reporter.DriftReporter) *Builder {
	b.overrides["reporter"] = r
	return b
}

// Build creates a container with the configured components.
// Components with overrides use the provided implementations;
// others are created using the factory.
func (b *Builder) Build(ctx context.Context) (*Container, error) {
	c := &Container{}

	// EC2 Repository
	if r, ok := b.overrides["ec2repo"].(repository.EC2Repository); ok {
		c.EC2Repository = r
	} else {
		repo, err := b.factory.CreateEC2Repository(ctx)
		if err != nil {
			return nil, err
		}
		c.EC2Repository = repo
	}

	// Terraform Repository
	if r, ok := b.overrides["tfrepo"].(repository.TerraformRepository); ok {
		c.TFRepository = r
	} else {
		c.TFRepository = b.factory.CreateTerraformRepository()
	}

	// Detector
	if d, ok := b.overrides["detector"].(drift.Detector); ok {
		c.Detector = d
	} else {
		c.Detector = b.factory.CreateDetector()
	}

	// Reporter - optional, may be nil if not needed
	if r, ok := b.overrides["reporter"].(reporter.DriftReporter); ok {
		c.Reporter = r
	}

	return c, nil
}

// CreateDriftService creates a DriftService from the container's components.
func (c *Container) CreateDriftService() *DriftService {
	return NewDriftService(c.EC2Repository, c.TFRepository, c.Detector)
}
