package aws

import (
	"context"
	"errors"
	"testing"

	"github.com/solomon-os/go-test/internal/models"
	"github.com/solomon-os/go-test/internal/repository"
)

// mockEC2Client implements aws.EC2Client for testing.
type mockEC2Client struct {
	instances map[string]*models.EC2Instance
	getErr    error
}

func (m *mockEC2Client) GetInstance(ctx context.Context, instanceID string) (*models.EC2Instance, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	inst, ok := m.instances[instanceID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return inst, nil
}

func (m *mockEC2Client) GetInstances(ctx context.Context, instanceIDs []string) ([]*models.EC2Instance, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	var result []*models.EC2Instance
	for _, id := range instanceIDs {
		if inst, ok := m.instances[id]; ok {
			result = append(result, inst)
		}
	}
	return result, nil
}

func TestNewEC2Repository(t *testing.T) {
	t.Run("creates repository with client", func(t *testing.T) {
		// Note: We can't easily create a mock aws.Client, but we can verify
		// the constructor doesn't panic
		repo := NewEC2Repository(nil)
		if repo == nil {
			t.Error("expected non-nil repository")
		}
	})
}

func TestEC2Repository_GetByID(t *testing.T) {
	t.Run("returns error for empty ID", func(t *testing.T) {
		repo := NewEC2Repository(nil)

		_, err := repo.GetByID(context.Background(), "")
		if !errors.Is(err, repository.ErrInvalidID) {
			t.Errorf("expected ErrInvalidID, got %v", err)
		}
	})

	// Note: Further tests would require proper mocking of aws.Client
	// which would need interface changes or dependency injection
}

func TestEC2Repository_GetByIDs(t *testing.T) {
	t.Run("returns empty slice for empty IDs", func(t *testing.T) {
		repo := NewEC2Repository(nil)

		result, err := repo.GetByIDs(context.Background(), []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty result, got %d", len(result))
		}
	})

	t.Run("returns empty slice for nil IDs", func(t *testing.T) {
		repo := NewEC2Repository(nil)

		result, err := repo.GetByIDs(context.Background(), nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty result, got %d", len(result))
		}
	})
}

func TestEC2Repository_List(t *testing.T) {
	t.Run("returns nil (not implemented)", func(t *testing.T) {
		repo := NewEC2Repository(nil)

		result, err := repo.List(context.Background())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != nil {
			t.Error("expected nil result for unimplemented List")
		}
	})
}

func TestEC2Repository_Client(t *testing.T) {
	t.Run("returns underlying client", func(t *testing.T) {
		repo := NewEC2Repository(nil)

		client := repo.Client()
		if client != nil {
			t.Error("expected nil client when initialized with nil")
		}
	})
}
