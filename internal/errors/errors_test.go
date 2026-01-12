package errors

import (
	"errors"
	"testing"
)

func TestBaseError(t *testing.T) {
	t.Run("New creates error with category and message", func(t *testing.T) {
		err := New(CategoryAWS, "test error")

		if err.Category() != CategoryAWS {
			t.Errorf("expected category %s, got %s", CategoryAWS, err.Category())
		}
		if err.Error() != "test error" {
			t.Errorf("expected message 'test error', got %s", err.Error())
		}
		if err.IsRetryable() {
			t.Error("expected non-retryable by default")
		}
	})

	t.Run("Newf creates error with formatted message", func(t *testing.T) {
		err := Newf(CategoryTerraform, "error: %s %d", "test", 42)

		if err.Error() != "error: test 42" {
			t.Errorf("expected formatted message, got %s", err.Error())
		}
	})

	t.Run("WithCause adds cause to error", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := New(CategoryDrift, "wrapper").WithCause(cause)

		if err.Unwrap() != cause {
			t.Error("expected cause to be set")
		}
		if err.Error() != "wrapper: underlying error" {
			t.Errorf("expected error with cause, got %s", err.Error())
		}
	})

	t.Run("WithRetryable sets retryable flag", func(t *testing.T) {
		err := New(CategoryAWS, "retryable error").WithRetryable(true)

		if !err.IsRetryable() {
			t.Error("expected error to be retryable")
		}
	})

	t.Run("WithMessage updates message", func(t *testing.T) {
		err := New(CategoryConfig, "original").WithMessage("updated")

		if err.Error() != "updated" {
			t.Errorf("expected 'updated', got %s", err.Error())
		}
	})
}

func TestWrap(t *testing.T) {
	t.Run("Wrap creates error with cause", func(t *testing.T) {
		cause := errors.New("original")
		err := Wrap(cause, CategoryAWS, "wrapped")

		if err.Category() != CategoryAWS {
			t.Errorf("expected category %s, got %s", CategoryAWS, err.Category())
		}
		if err.Unwrap() != cause {
			t.Error("expected cause to be wrapped")
		}
	})

	t.Run("Wrapf creates error with formatted message", func(t *testing.T) {
		cause := errors.New("original")
		err := Wrapf(cause, CategoryTerraform, "wrapped: %s", "detail")

		if err.Error() != "wrapped: detail: original" {
			t.Errorf("expected formatted wrapped message, got %s", err.Error())
		}
	})
}

func TestIsRetryable(t *testing.T) {
	t.Run("returns true for retryable DriftError", func(t *testing.T) {
		err := New(CategoryAWS, "retryable").WithRetryable(true)

		if !IsRetryable(err) {
			t.Error("expected IsRetryable to return true")
		}
	})

	t.Run("returns false for non-retryable DriftError", func(t *testing.T) {
		err := New(CategoryAWS, "not retryable")

		if IsRetryable(err) {
			t.Error("expected IsRetryable to return false")
		}
	})

	t.Run("returns false for non-DriftError", func(t *testing.T) {
		err := errors.New("standard error")

		if IsRetryable(err) {
			t.Error("expected IsRetryable to return false for standard error")
		}
	})
}

func TestGetCategory(t *testing.T) {
	t.Run("returns category for DriftError", func(t *testing.T) {
		err := New(CategoryTerraform, "test")

		cat, ok := GetCategory(err)
		if !ok {
			t.Error("expected GetCategory to succeed")
		}
		if cat != CategoryTerraform {
			t.Errorf("expected %s, got %s", CategoryTerraform, cat)
		}
	})

	t.Run("returns false for non-DriftError", func(t *testing.T) {
		err := errors.New("standard error")

		_, ok := GetCategory(err)
		if ok {
			t.Error("expected GetCategory to return false for standard error")
		}
	})
}

func TestAggregateError(t *testing.T) {
	t.Run("NewAggregateError creates error with multiple errors", func(t *testing.T) {
		errs := []error{
			errors.New("error 1"),
			errors.New("error 2"),
			errors.New("error 3"),
		}
		aggErr := NewAggregateError(CategoryDrift, "multiple errors", errs)

		if !aggErr.HasErrors() {
			t.Error("expected HasErrors to return true")
		}
		if len(aggErr.Errors) != 3 {
			t.Errorf("expected 3 errors, got %d", len(aggErr.Errors))
		}
	})

	t.Run("Error returns formatted message for multiple errors", func(t *testing.T) {
		errs := []error{
			errors.New("error 1"),
			errors.New("error 2"),
		}
		aggErr := NewAggregateError(CategoryDrift, "failed", errs)

		expected := "failed: 2 errors occurred"
		if aggErr.Error() != expected {
			t.Errorf("expected %q, got %q", expected, aggErr.Error())
		}
	})

	t.Run("Error returns single error message for one error", func(t *testing.T) {
		errs := []error{errors.New("single error")}
		aggErr := NewAggregateError(CategoryDrift, "failed", errs)

		expected := "failed: single error"
		if aggErr.Error() != expected {
			t.Errorf("expected %q, got %q", expected, aggErr.Error())
		}
	})

	t.Run("First returns first error", func(t *testing.T) {
		first := errors.New("first")
		errs := []error{first, errors.New("second")}
		aggErr := NewAggregateError(CategoryDrift, "failed", errs)

		if aggErr.First() != first {
			t.Error("expected First to return first error")
		}
	})

	t.Run("First returns nil for empty aggregate", func(t *testing.T) {
		aggErr := NewAggregateError(CategoryDrift, "empty", nil)

		if aggErr.First() != nil {
			t.Error("expected First to return nil for empty aggregate")
		}
	})
}

func TestSentinelErrors(t *testing.T) {
	t.Run("ErrNotFound has correct category", func(t *testing.T) {
		if ErrNotFound.Category() != CategoryInternal {
			t.Errorf("expected CategoryInternal, got %s", ErrNotFound.Category())
		}
	})

	t.Run("ErrTimeout is retryable", func(t *testing.T) {
		if !ErrTimeout.IsRetryable() {
			t.Error("expected ErrTimeout to be retryable")
		}
	})

	t.Run("errors.Is works with sentinel errors", func(t *testing.T) {
		err := Wrap(ErrNotFound, CategoryAWS, "wrapped")

		if !Is(err, ErrNotFound) {
			t.Error("expected Is to find wrapped sentinel error")
		}
	})
}

func TestErrorsAs(t *testing.T) {
	t.Run("As finds BaseError in chain", func(t *testing.T) {
		baseErr := New(CategoryAWS, "base error")
		wrappedErr := Wrap(baseErr, CategoryDrift, "wrapped")

		var target *BaseError
		if !As(wrappedErr, &target) {
			t.Error("expected As to find BaseError")
		}
	})
}
