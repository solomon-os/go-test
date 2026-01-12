// Package errors provides structured error types for the EC2 drift detector.
//
// This package defines a common error interface and base types that all
// domain-specific errors in the application implement. The structured
// approach enables:
//   - Categorization of errors by domain (AWS, Terraform, config, drift)
//   - Identification of retryable vs non-retryable errors
//   - Error wrapping for cause chain inspection
//   - Consistent error handling patterns across the codebase
//
// Example usage:
//
//	err := errors.New(errors.CategoryAWS, "failed to describe instance").
//	    WithCause(originalErr).
//	    WithRetryable(true)
//	if errors.IsRetryable(err) {
//	    // retry the operation
//	}
package errors

import (
	"errors"
	"fmt"
)

// Category represents the error category for classification.
type Category string

// Error categories for the drift detector.
const (
	CategoryAWS       Category = "aws"
	CategoryTerraform Category = "terraform"
	CategoryConfig    Category = "config"
	CategoryDrift     Category = "drift"
	CategoryInternal  Category = "internal"
)

// DriftError is the base interface for all drift detector errors.
// All structured errors in the application implement this interface.
type DriftError interface {
	error
	// Category returns the error category (aws, terraform, config, drift).
	Category() Category
	// Unwrap returns the underlying cause of the error.
	Unwrap() error
	// IsRetryable indicates whether the operation that caused this error
	// can be safely retried.
	IsRetryable() bool
}

// BaseError provides common error functionality.
// Embed this struct in domain-specific error types.
type BaseError struct {
	category  Category
	message   string
	cause     error
	retryable bool
}

// New creates a new BaseError with the specified category and message.
func New(category Category, message string) *BaseError {
	return &BaseError{
		category: category,
		message:  message,
	}
}

// Newf creates a new BaseError with a formatted message.
func Newf(category Category, format string, args ...any) *BaseError {
	return &BaseError{
		category: category,
		message:  fmt.Sprintf(format, args...),
	}
}

// Error implements the error interface.
func (e *BaseError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.message, e.cause)
	}
	return e.message
}

// Category returns the error category.
func (e *BaseError) Category() Category {
	return e.category
}

// Unwrap returns the underlying cause of the error.
func (e *BaseError) Unwrap() error {
	return e.cause
}

// IsRetryable indicates whether the operation can be retried.
func (e *BaseError) IsRetryable() bool {
	return e.retryable
}

// WithCause sets the underlying cause of the error.
func (e *BaseError) WithCause(cause error) *BaseError {
	e.cause = cause
	return e
}

// WithRetryable sets whether the error is retryable.
func (e *BaseError) WithRetryable(retryable bool) *BaseError {
	e.retryable = retryable
	return e
}

// WithMessage updates the error message.
func (e *BaseError) WithMessage(message string) *BaseError {
	e.message = message
	return e
}

// Wrap wraps an error with additional context and a category.
func Wrap(err error, category Category, message string) *BaseError {
	return &BaseError{
		category: category,
		message:  message,
		cause:    err,
	}
}

// Wrapf wraps an error with a formatted message and a category.
func Wrapf(err error, category Category, format string, args ...any) *BaseError {
	return &BaseError{
		category:  category,
		message:   fmt.Sprintf(format, args...),
		cause:     err,
		retryable: false,
	}
}

// IsRetryable checks if any error in the chain is retryable.
func IsRetryable(err error) bool {
	var driftErr DriftError
	if errors.As(err, &driftErr) {
		return driftErr.IsRetryable()
	}
	return false
}

// GetCategory returns the category of the error if it's a DriftError.
func GetCategory(err error) (Category, bool) {
	var driftErr DriftError
	if errors.As(err, &driftErr) {
		return driftErr.Category(), true
	}
	return "", false
}

// Is reports whether any error in err's chain matches target.
// This is a convenience wrapper around errors.Is.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's chain that matches target.
// This is a convenience wrapper around errors.As.
func As(err error, target any) bool {
	return errors.As(err, target)
}

// AggregateError collects multiple errors from parallel operations.
type AggregateError struct {
	BaseError
	Errors []error
}

// NewAggregateError creates a new AggregateError with the given errors.
func NewAggregateError(category Category, message string, errs []error) *AggregateError {
	return &AggregateError{
		BaseError: BaseError{
			category: category,
			message:  message,
		},
		Errors: errs,
	}
}

// Error implements the error interface with details about all errors.
func (e *AggregateError) Error() string {
	if len(e.Errors) == 0 {
		return e.message
	}
	if len(e.Errors) == 1 {
		return fmt.Sprintf("%s: %v", e.message, e.Errors[0])
	}
	return fmt.Sprintf("%s: %d errors occurred", e.message, len(e.Errors))
}

// HasErrors returns true if there are any errors in the aggregate.
func (e *AggregateError) HasErrors() bool {
	return len(e.Errors) > 0
}

// First returns the first error in the aggregate, or nil if empty.
func (e *AggregateError) First() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e.Errors[0]
}

// Sentinel errors for common conditions.
var (
	// ErrNotFound indicates a requested resource was not found.
	ErrNotFound = New(CategoryInternal, "resource not found")

	// ErrInvalidInput indicates invalid input was provided.
	ErrInvalidInput = New(CategoryConfig, "invalid input")

	// ErrTimeout indicates an operation timed out.
	ErrTimeout = New(CategoryInternal, "operation timed out").WithRetryable(true)

	// ErrCanceled indicates an operation was canceled.
	ErrCanceled = New(CategoryInternal, "operation canceled")
)
