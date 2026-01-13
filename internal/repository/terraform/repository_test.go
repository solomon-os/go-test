package terraform

import (
	"context"
	"errors"
	"testing"

	"github.com/solomon-os/go-test/internal/models"
	"github.com/solomon-os/go-test/internal/repository"
)

// mockParser implements terraform.StateParser for testing.
type mockParser struct {
	instances map[string]*models.EC2Instance
	parseErr  error
}

func (m *mockParser) ParseFile(filePath string) (map[string]*models.EC2Instance, error) {
	if m.parseErr != nil {
		return nil, m.parseErr
	}
	return m.instances, nil
}

func (m *mockParser) ParseStateFile(filePath string) (map[string]*models.EC2Instance, error) {
	return m.ParseFile(filePath)
}

func (m *mockParser) ParseStateJSON(data []byte) (map[string]*models.EC2Instance, error) {
	return m.instances, m.parseErr
}

func (m *mockParser) ParseHCLFile(filePath string) (map[string]*models.EC2Instance, error) {
	return m.ParseFile(filePath)
}

func (m *mockParser) ParseHCL(data []byte, filename string) (map[string]*models.EC2Instance, error) {
	return m.instances, m.parseErr
}

func (m *mockParser) GetInstanceByID(instances map[string]*models.EC2Instance, instanceID string) (*models.EC2Instance, error) {
	inst, ok := instances[instanceID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return inst, nil
}

func TestNewRepository(t *testing.T) {
	parser := &mockParser{}
	repo := NewRepository(parser, "/path/to/state.tfstate")

	if repo.FilePath() != "/path/to/state.tfstate" {
		t.Errorf("expected file path '/path/to/state.tfstate', got %s", repo.FilePath())
	}

	if repo.IsLoaded() {
		t.Error("expected repository to not be loaded initially")
	}
}

func TestRepository_GetByID(t *testing.T) {
	t.Run("returns error for empty ID", func(t *testing.T) {
		parser := &mockParser{}
		repo := NewRepository(parser, "/path/to/state.tfstate")

		_, err := repo.GetByID(context.Background(), "")
		if !errors.Is(err, repository.ErrInvalidID) {
			t.Errorf("expected ErrInvalidID, got %v", err)
		}
	})

	t.Run("loads and returns instance", func(t *testing.T) {
		instances := map[string]*models.EC2Instance{
			"i-123": {InstanceID: "i-123", InstanceType: "t2.micro"},
		}
		parser := &mockParser{instances: instances}
		repo := NewRepository(parser, "/path/to/state.tfstate")

		inst, err := repo.GetByID(context.Background(), "i-123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if inst.InstanceID != "i-123" {
			t.Errorf("expected instance ID 'i-123', got %s", inst.InstanceID)
		}
		if inst.InstanceType != "t2.micro" {
			t.Errorf("expected instance type 't2.micro', got %s", inst.InstanceType)
		}
	})

	t.Run("returns error for non-existent instance", func(t *testing.T) {
		parser := &mockParser{instances: map[string]*models.EC2Instance{}}
		repo := NewRepository(parser, "/path/to/state.tfstate")

		_, err := repo.GetByID(context.Background(), "i-nonexistent")
		if !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestRepository_GetAll(t *testing.T) {
	t.Run("loads and returns all instances", func(t *testing.T) {
		instances := map[string]*models.EC2Instance{
			"i-123": {InstanceID: "i-123"},
			"i-456": {InstanceID: "i-456"},
		}
		parser := &mockParser{instances: instances}
		repo := NewRepository(parser, "/path/to/state.tfstate")

		result, err := repo.GetAll(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 instances, got %d", len(result))
		}
	})

	t.Run("returns copy of instances", func(t *testing.T) {
		instances := map[string]*models.EC2Instance{
			"i-123": {InstanceID: "i-123"},
		}
		parser := &mockParser{instances: instances}
		repo := NewRepository(parser, "/path/to/state.tfstate")

		result1, _ := repo.GetAll(context.Background())
		result2, _ := repo.GetAll(context.Background())

		// Modify result1
		delete(result1, "i-123")

		// result2 should still have the instance
		if len(result2) != 1 {
			t.Error("expected GetAll to return a copy, not the original map")
		}
	})
}

func TestRepository_Refresh(t *testing.T) {
	t.Run("reloads data from parser", func(t *testing.T) {
		instances := map[string]*models.EC2Instance{
			"i-123": {InstanceID: "i-123"},
		}
		parser := &mockParser{instances: instances}
		repo := NewRepository(parser, "/path/to/state.tfstate")

		// Initial load
		_, _ = repo.GetAll(context.Background())
		if repo.InstanceCount() != 1 {
			t.Errorf("expected 1 instance, got %d", repo.InstanceCount())
		}

		// Update mock and refresh
		parser.instances = map[string]*models.EC2Instance{
			"i-123": {InstanceID: "i-123"},
			"i-456": {InstanceID: "i-456"},
		}

		err := repo.Refresh(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if repo.InstanceCount() != 2 {
			t.Errorf("expected 2 instances after refresh, got %d", repo.InstanceCount())
		}
	})

	t.Run("returns error on parse failure", func(t *testing.T) {
		parser := &mockParser{parseErr: repository.ErrNotFound}
		repo := NewRepository(parser, "/path/to/state.tfstate")

		err := repo.Refresh(context.Background())
		if err == nil {
			t.Error("expected error on parse failure")
		}
	})
}

func TestRepository_IsLoaded(t *testing.T) {
	t.Run("returns false before load", func(t *testing.T) {
		parser := &mockParser{instances: map[string]*models.EC2Instance{}}
		repo := NewRepository(parser, "/path/to/state.tfstate")

		if repo.IsLoaded() {
			t.Error("expected IsLoaded to return false before load")
		}
	})

	t.Run("returns true after load", func(t *testing.T) {
		parser := &mockParser{instances: map[string]*models.EC2Instance{}}
		repo := NewRepository(parser, "/path/to/state.tfstate")

		_, _ = repo.GetAll(context.Background())

		if !repo.IsLoaded() {
			t.Error("expected IsLoaded to return true after load")
		}
	})
}
