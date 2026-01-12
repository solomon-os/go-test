package factory

import (
	"context"
	"testing"
	"time"

	"github.com/solomon-os/go-test/internal/drift"
	"github.com/solomon-os/go-test/internal/models"
	"github.com/solomon-os/go-test/internal/repository"
	"github.com/solomon-os/go-test/internal/retry"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	t.Run("has valid AWS region", func(t *testing.T) {
		if cfg.AWSRegion == "" {
			t.Error("expected non-empty AWS region")
		}
	})

	t.Run("has default output format", func(t *testing.T) {
		if cfg.OutputFormat == "" {
			t.Error("expected non-empty output format")
		}
	})

	t.Run("has positive concurrency", func(t *testing.T) {
		if cfg.Concurrency <= 0 {
			t.Error("expected positive concurrency")
		}
	})

	t.Run("has valid retry config", func(t *testing.T) {
		if cfg.RetryConfig.MaxAttempts <= 0 {
			t.Error("expected positive max attempts")
		}
		if cfg.RetryConfig.InitialDelay <= 0 {
			t.Error("expected positive initial delay")
		}
	})
}

func TestNew(t *testing.T) {
	t.Run("creates factory with config", func(t *testing.T) {
		cfg := Config{
			AWSRegion:    "us-west-2",
			OutputFormat: "json",
			Concurrency:  20,
		}

		f := New(cfg)

		if f.Config().AWSRegion != "us-west-2" {
			t.Errorf("expected region 'us-west-2', got %s", f.Config().AWSRegion)
		}
		if f.Config().OutputFormat != "json" {
			t.Errorf("expected format 'json', got %s", f.Config().OutputFormat)
		}
		if f.Config().Concurrency != 20 {
			t.Errorf("expected concurrency 20, got %d", f.Config().Concurrency)
		}
	})
}

func TestFactory_CreateParser(t *testing.T) {
	f := New(DefaultConfig())

	t.Run("creates parser", func(t *testing.T) {
		parser := f.CreateParser()
		if parser == nil {
			t.Error("expected non-nil parser")
		}
	})

	t.Run("returns cached parser on subsequent calls", func(t *testing.T) {
		parser1 := f.CreateParser()
		parser2 := f.CreateParser()

		// Should return the same instance
		if parser1 != parser2 {
			t.Error("expected cached parser to be returned")
		}
	})
}

func TestFactory_CreateDetector(t *testing.T) {
	t.Run("creates detector with default attributes", func(t *testing.T) {
		cfg := DefaultConfig()
		f := New(cfg)

		detector := f.CreateDetector()
		if detector == nil {
			t.Error("expected non-nil detector")
		}

		attrs := detector.GetAttributes()
		if len(attrs) == 0 {
			t.Error("expected default attributes")
		}
	})

	t.Run("creates detector with custom attributes", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Attributes = []string{"instance_type", "ami"}
		f := New(cfg)

		detector := f.CreateDetector()
		attrs := detector.GetAttributes()

		if len(attrs) != 2 {
			t.Errorf("expected 2 attributes, got %d", len(attrs))
		}
	})

	t.Run("creates detector with custom concurrency", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Concurrency = 50
		f := New(cfg)

		detector := f.CreateDetector()

		// Check if detector respects concurrency
		// We need to cast to DefaultDetector to access Concurrency method
		if dd, ok := detector.(*drift.DefaultDetector); ok {
			if dd.Concurrency() != 50 {
				t.Errorf("expected concurrency 50, got %d", dd.Concurrency())
			}
		}
	})
}

func TestFactory_CreateTerraformRepository(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TerraformPath = "/path/to/state.tfstate"
	f := New(cfg)

	t.Run("creates terraform repository", func(t *testing.T) {
		repo := f.CreateTerraformRepository()
		if repo == nil {
			t.Error("expected non-nil repository")
		}

		if repo.FilePath() != "/path/to/state.tfstate" {
			t.Errorf("expected file path '/path/to/state.tfstate', got %s", repo.FilePath())
		}
	})
}

func TestFactory_FormattersRegistry(t *testing.T) {
	f := New(DefaultConfig())

	t.Run("returns formatter registry", func(t *testing.T) {
		reg := f.FormattersRegistry()
		if reg == nil {
			t.Error("expected non-nil registry")
		}

		// Verify built-in formatters are available
		if _, ok := reg.Get("json"); !ok {
			t.Error("expected json formatter")
		}
		if _, ok := reg.Get("text"); !ok {
			t.Error("expected text formatter")
		}
	})
}

func TestFactory_CreateFormatter(t *testing.T) {
	t.Run("returns formatter for configured format", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.OutputFormat = "json"
		f := New(cfg)

		formatter, ok := f.CreateFormatter()
		if !ok {
			t.Error("expected formatter to be found")
		}
		if formatter.Name() != "json" {
			t.Errorf("expected json formatter, got %s", formatter.Name())
		}
	})

	t.Run("returns false for unknown format", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.OutputFormat = "unknown"
		f := New(cfg)

		_, ok := f.CreateFormatter()
		if ok {
			t.Error("expected formatter not to be found for unknown format")
		}
	})
}

func TestNewDriftService(t *testing.T) {
	t.Run("creates service with dependencies", func(t *testing.T) {
		// Use mock implementations
		awsRepo := &mockEC2Repository{}
		tfRepo := &mockTerraformRepository{}
		detector := drift.NewDetector(nil)

		service := NewDriftService(awsRepo, tfRepo, detector)

		if service == nil {
			t.Error("expected non-nil service")
		}
		if service.Detector() != detector {
			t.Error("expected same detector")
		}
	})
}

func TestConfig_WithRetryConfig(t *testing.T) {
	cfg := DefaultConfig()
	customRetry := retry.Config{
		MaxAttempts:  5,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     1 * time.Minute,
		Multiplier:   3.0,
	}
	cfg.RetryConfig = customRetry

	if cfg.RetryConfig.MaxAttempts != 5 {
		t.Errorf("expected max attempts 5, got %d", cfg.RetryConfig.MaxAttempts)
	}
	if cfg.RetryConfig.Multiplier != 3.0 {
		t.Errorf("expected multiplier 3.0, got %f", cfg.RetryConfig.Multiplier)
	}
}

// Mock implementations for testing
type mockEC2Repository struct{}

func (m *mockEC2Repository) GetByID(ctx context.Context, id string) (*models.EC2Instance, error) {
	return nil, nil
}

func (m *mockEC2Repository) GetByIDs(ctx context.Context, ids []string) ([]*models.EC2Instance, error) {
	return nil, nil
}

func (m *mockEC2Repository) List(ctx context.Context, filters ...repository.Filter) ([]*models.EC2Instance, error) {
	return nil, nil
}

type mockTerraformRepository struct{}

func (m *mockTerraformRepository) GetByID(ctx context.Context, id string) (*models.EC2Instance, error) {
	return nil, nil
}

func (m *mockTerraformRepository) GetAll(ctx context.Context) (map[string]*models.EC2Instance, error) {
	return nil, nil
}

func (m *mockTerraformRepository) Refresh(ctx context.Context) error { return nil }
func (m *mockTerraformRepository) FilePath() string                  { return "/mock/path" }
