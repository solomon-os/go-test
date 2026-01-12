// Package drift provides functionality to detect configuration drift between AWS and Terraform.
package drift

import (
	"fmt"
	"strings"

	"github.com/solomon-os/go-test/internal/errors"
)

// DetectionError represents errors that occur during drift detection.
// It includes context about the instance and attribute being checked
// when the error occurred.
type DetectionError struct {
	errors.BaseError
	// InstanceID is the EC2 instance ID being checked.
	InstanceID string
	// Attribute is the attribute being compared when the error occurred.
	Attribute string
	// Phase indicates where in the detection process the error occurred.
	// Common values: "extraction", "comparison", "initialization"
	Phase string
}

// NewDetectionError creates a new DetectionError.
func NewDetectionError(instanceID, attribute, phase string, cause error) *DetectionError {
	return &DetectionError{
		BaseError: *errors.Wrap(cause, errors.CategoryDrift,
			fmt.Sprintf("drift detection failed for %s", instanceID)),
		InstanceID: instanceID,
		Attribute:  attribute,
		Phase:      phase,
	}
}

// Error implements the error interface with detailed context.
func (e *DetectionError) Error() string {
	var parts []string
	parts = append(parts, "drift detection failed")
	if e.InstanceID != "" {
		parts = append(parts, fmt.Sprintf("for instance %s", e.InstanceID))
	}
	if e.Attribute != "" {
		parts = append(parts, fmt.Sprintf("on attribute %s", e.Attribute))
	}
	if e.Phase != "" {
		parts = append(parts, fmt.Sprintf("during %s", e.Phase))
	}
	msg := strings.Join(parts, " ")
	if cause := e.Unwrap(); cause != nil {
		msg += fmt.Sprintf(": %v", cause)
	}
	return msg
}

// AttributeError represents errors related to attribute extraction or comparison.
type AttributeError struct {
	errors.BaseError
	// Attribute is the attribute path that caused the error.
	Attribute string
	// Source indicates where the attribute was being extracted from.
	// Common values: "aws", "terraform"
	Source string
}

// NewAttributeError creates a new AttributeError.
func NewAttributeError(attribute, source string, cause error) *AttributeError {
	return &AttributeError{
		BaseError: *errors.Wrap(cause, errors.CategoryDrift,
			fmt.Sprintf("failed to extract attribute %s from %s", attribute, source)),
		Attribute: attribute,
		Source:    source,
	}
}

// Error implements the error interface.
func (e *AttributeError) Error() string {
	msg := fmt.Sprintf("attribute error for %s", e.Attribute)
	if e.Source != "" {
		msg += fmt.Sprintf(" from %s", e.Source)
	}
	if cause := e.Unwrap(); cause != nil {
		msg += fmt.Sprintf(": %v", cause)
	}
	return msg
}

// ComparisonError represents errors that occur during value comparison.
type ComparisonError struct {
	errors.BaseError
	// Attribute is the attribute being compared.
	Attribute string
	// AWSValue is the value from AWS.
	AWSValue interface{}
	// TerraformValue is the value from Terraform.
	TerraformValue interface{}
}

// NewComparisonError creates a new ComparisonError.
func NewComparisonError(attribute string, awsVal, tfVal interface{}, cause error) *ComparisonError {
	return &ComparisonError{
		BaseError: *errors.Wrap(cause, errors.CategoryDrift,
			fmt.Sprintf("comparison failed for attribute %s", attribute)),
		Attribute:      attribute,
		AWSValue:       awsVal,
		TerraformValue: tfVal,
	}
}

// Error implements the error interface.
func (e *ComparisonError) Error() string {
	return fmt.Sprintf("comparison error for %s: AWS=%v, Terraform=%v",
		e.Attribute, e.AWSValue, e.TerraformValue)
}

// ConfigurationError represents errors in detector configuration.
type ConfigurationError struct {
	errors.BaseError
	// Field is the configuration field that has an issue.
	Field string
	// Value is the invalid value.
	Value interface{}
	// Reason explains why the configuration is invalid.
	Reason string
}

// NewConfigurationError creates a new ConfigurationError.
func NewConfigurationError(field string, value interface{}, reason string) *ConfigurationError {
	return &ConfigurationError{
		BaseError: *errors.New(errors.CategoryConfig,
			fmt.Sprintf("invalid configuration for %s", field)),
		Field:  field,
		Value:  value,
		Reason: reason,
	}
}

// Error implements the error interface.
func (e *ConfigurationError) Error() string {
	msg := fmt.Sprintf("configuration error for %s", e.Field)
	if e.Value != nil {
		msg += fmt.Sprintf("=%v", e.Value)
	}
	if e.Reason != "" {
		msg += fmt.Sprintf(": %s", e.Reason)
	}
	return msg
}

// BatchError collects multiple errors from concurrent drift detection operations.
type BatchError struct {
	errors.BaseError
	// Errors is the list of individual errors that occurred.
	Errors []error
	// FailedInstances is the list of instance IDs that failed.
	FailedInstances []string
}

// NewBatchError creates a new BatchError from multiple errors.
func NewBatchError(errs []error) *BatchError {
	if len(errs) == 0 {
		return nil
	}

	failedInstances := make([]string, 0)
	for _, err := range errs {
		var detErr *DetectionError
		if errors.As(err, &detErr) && detErr.InstanceID != "" {
			failedInstances = append(failedInstances, detErr.InstanceID)
		}
	}

	return &BatchError{
		BaseError: *errors.New(errors.CategoryDrift,
			fmt.Sprintf("drift detection failed for %d instances", len(errs))),
		Errors:          errs,
		FailedInstances: failedInstances,
	}
}

// Error implements the error interface.
func (e *BatchError) Error() string {
	if len(e.Errors) == 0 {
		return "no errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	return fmt.Sprintf("%d drift detection errors occurred", len(e.Errors))
}

// HasErrors returns true if there are any errors.
func (e *BatchError) HasErrors() bool {
	return len(e.Errors) > 0
}

// First returns the first error, or nil if empty.
func (e *BatchError) First() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e.Errors[0]
}

// Sentinel errors for common drift detection conditions.
var (
	// ErrInstanceNotFound indicates the instance was not found in the source data.
	ErrInstanceNotFound = errors.New(errors.CategoryDrift, "instance not found")

	// ErrAttributeNotFound indicates the attribute was not found on the instance.
	ErrAttributeNotFound = errors.New(errors.CategoryDrift, "attribute not found")

	// ErrInvalidAttributePath indicates the attribute path is malformed.
	ErrInvalidAttributePath = errors.New(errors.CategoryDrift, "invalid attribute path")

	// ErrNilInstance indicates a nil instance was provided.
	ErrNilInstance = errors.New(errors.CategoryDrift, "nil instance provided")
)

// Ensure error types implement the DriftError interface.
var (
	_ errors.DriftError = (*DetectionError)(nil)
	_ errors.DriftError = (*AttributeError)(nil)
	_ errors.DriftError = (*ComparisonError)(nil)
	_ errors.DriftError = (*ConfigurationError)(nil)
	_ errors.DriftError = (*BatchError)(nil)
)
