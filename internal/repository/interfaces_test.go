package repository

import (
	"testing"
)

func TestNewFilter(t *testing.T) {
	t.Run("creates filter with name and values", func(t *testing.T) {
		f := NewFilter("instance-type", "t2.micro", "t2.small")

		if f.Name != "instance-type" {
			t.Errorf("expected name 'instance-type', got %s", f.Name)
		}
		if len(f.Values) != 2 {
			t.Errorf("expected 2 values, got %d", len(f.Values))
		}
		if f.Values[0] != "t2.micro" {
			t.Errorf("expected first value 't2.micro', got %s", f.Values[0])
		}
	})
}

func TestTagFilter(t *testing.T) {
	t.Run("creates tag filter", func(t *testing.T) {
		f := TagFilter("Environment", "production")

		if f.Name != "tag:Environment" {
			t.Errorf("expected name 'tag:Environment', got %s", f.Name)
		}
		if len(f.Values) != 1 || f.Values[0] != "production" {
			t.Error("expected single value 'production'")
		}
	})
}

func TestInstanceTypeFilter(t *testing.T) {
	t.Run("creates instance type filter", func(t *testing.T) {
		f := InstanceTypeFilter("t2.micro")

		if f.Name != "instance-type" {
			t.Errorf("expected name 'instance-type', got %s", f.Name)
		}
		if len(f.Values) != 1 || f.Values[0] != "t2.micro" {
			t.Error("expected single value 't2.micro'")
		}
	})
}

func TestPredefinedFilters(t *testing.T) {
	t.Run("FilterRunning has correct values", func(t *testing.T) {
		if FilterRunning.Name != "instance-state-name" {
			t.Errorf("expected name 'instance-state-name', got %s", FilterRunning.Name)
		}
		if len(FilterRunning.Values) != 1 || FilterRunning.Values[0] != "running" {
			t.Error("expected single value 'running'")
		}
	})

	t.Run("FilterStopped has correct values", func(t *testing.T) {
		if FilterStopped.Name != "instance-state-name" {
			t.Errorf("expected name 'instance-state-name', got %s", FilterStopped.Name)
		}
		if len(FilterStopped.Values) != 1 || FilterStopped.Values[0] != "stopped" {
			t.Error("expected single value 'stopped'")
		}
	})
}

func TestSentinelErrors(t *testing.T) {
	t.Run("ErrNotFound has correct message", func(t *testing.T) {
		if ErrNotFound.Error() != "resource not found" {
			t.Errorf("unexpected error message: %s", ErrNotFound.Error())
		}
	})

	t.Run("ErrNotLoaded has correct message", func(t *testing.T) {
		if ErrNotLoaded.Error() != "repository data not loaded" {
			t.Errorf("unexpected error message: %s", ErrNotLoaded.Error())
		}
	})

	t.Run("ErrInvalidID has correct message", func(t *testing.T) {
		if ErrInvalidID.Error() != "invalid resource ID" {
			t.Errorf("unexpected error message: %s", ErrInvalidID.Error())
		}
	})
}
