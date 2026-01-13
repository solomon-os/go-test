// Package terraform provides functionality to parse Terraform state and HCL files.
package terraform

import (
	"fmt"

	"github.com/solomon-os/go-test/internal/errors"
)

// ParseError represents errors that occur during Terraform file parsing.
// It includes information about the file, line number, and file type
// to help with debugging parsing issues.
type ParseError struct {
	errors.BaseError
	// FilePath is the path to the file that failed to parse.
	FilePath string
	// LineNumber is the line number where the error occurred (0 if unknown).
	LineNumber int
	// FileType is the type of file being parsed (e.g., "hcl", "json", "tfstate").
	FileType string
}

// NewParseError creates a new ParseError with the given details.
func NewParseError(filePath, fileType string, cause error) *ParseError {
	return &ParseError{
		BaseError: *errors.Wrap(cause, errors.CategoryTerraform,
			fmt.Sprintf("failed to parse %s file", fileType)),
		FilePath: filePath,
		FileType: fileType,
	}
}

// WithLineNumber sets the line number where the error occurred.
func (e *ParseError) WithLineNumber(line int) *ParseError {
	e.LineNumber = line
	return e
}

// Error implements the error interface with detailed context.
func (e *ParseError) Error() string {
	msg := fmt.Sprintf("failed to parse %s file %s", e.FileType, e.FilePath)
	if e.LineNumber > 0 {
		msg += fmt.Sprintf(" at line %d", e.LineNumber)
	}
	if cause := e.Unwrap(); cause != nil {
		msg += fmt.Sprintf(": %v", cause)
	}
	return msg
}

// ValidationError represents semantic validation errors in Terraform configuration.
// These errors occur when the configuration is syntactically valid but contains
// invalid or inconsistent values.
type ValidationError struct {
	errors.BaseError
	// ResourceName is the name of the resource that failed validation.
	ResourceName string
	// ResourceType is the type of the resource (e.g., "aws_instance").
	ResourceType string
	// Field is the specific field that has an invalid value.
	Field string
	// Expected describes what value was expected.
	Expected string
	// Actual is the actual value that was found.
	Actual string
}

// NewValidationError creates a new ValidationError.
func NewValidationError(
	resourceType, resourceName, field, expected, actual string,
) *ValidationError {
	return &ValidationError{
		BaseError: *errors.New(errors.CategoryTerraform,
			fmt.Sprintf("invalid value for %s.%s.%s", resourceType, resourceName, field)),
		ResourceName: resourceName,
		ResourceType: resourceType,
		Field:        field,
		Expected:     expected,
		Actual:       actual,
	}
}

// Error implements the error interface with detailed context.
func (e *ValidationError) Error() string {
	msg := fmt.Sprintf("validation error for %s.%s.%s", e.ResourceType, e.ResourceName, e.Field)
	if e.Expected != "" && e.Actual != "" {
		msg += fmt.Sprintf(": expected %s, got %s", e.Expected, e.Actual)
	} else if e.Actual != "" {
		msg += fmt.Sprintf(": invalid value %s", e.Actual)
	}
	return msg
}

// ResourceNotFoundError represents an error when a required resource is not found.
type ResourceNotFoundError struct {
	errors.BaseError
	// ResourceType is the type of resource that was not found.
	ResourceType string
	// ResourceID is the identifier of the resource.
	ResourceID string
	// FilePath is the file that was searched.
	FilePath string
}

// NewResourceNotFoundError creates a new ResourceNotFoundError.
func NewResourceNotFoundError(resourceType, resourceID, filePath string) *ResourceNotFoundError {
	return &ResourceNotFoundError{
		BaseError: *errors.New(errors.CategoryTerraform,
			fmt.Sprintf("%s %s not found in Terraform configuration", resourceType, resourceID)),
		ResourceType: resourceType,
		ResourceID:   resourceID,
		FilePath:     filePath,
	}
}

// Error implements the error interface.
func (e *ResourceNotFoundError) Error() string {
	msg := fmt.Sprintf("%s %s not found", e.ResourceType, e.ResourceID)
	if e.FilePath != "" {
		msg += fmt.Sprintf(" in %s", e.FilePath)
	}
	return msg
}

// FileTypeError represents an error when an unsupported file type is encountered.
type FileTypeError struct {
	errors.BaseError
	// FilePath is the path to the unsupported file.
	FilePath string
	// Extension is the file extension.
	Extension string
	// SupportedTypes lists the supported file types.
	SupportedTypes []string
}

// NewFileTypeError creates a new FileTypeError.
func NewFileTypeError(filePath, extension string, supportedTypes []string) *FileTypeError {
	return &FileTypeError{
		BaseError: *errors.New(errors.CategoryTerraform,
			fmt.Sprintf("unsupported file type: %s", extension)),
		FilePath:       filePath,
		Extension:      extension,
		SupportedTypes: supportedTypes,
	}
}

// Error implements the error interface.
func (e *FileTypeError) Error() string {
	return fmt.Sprintf("unsupported file type %q for %s (supported: %v)",
		e.Extension, e.FilePath, e.SupportedTypes)
}

// Sentinel errors for common Terraform parsing conditions.
var (
	// ErrEmptyState indicates the Terraform state file is empty or has no resources.
	ErrEmptyState = errors.New(errors.CategoryTerraform, "terraform state is empty")

	// ErrInvalidState indicates the Terraform state file has an invalid structure.
	ErrInvalidState = errors.New(errors.CategoryTerraform, "invalid terraform state structure")

	// ErrNoInstances indicates no EC2 instances were found in the configuration.
	ErrNoInstances = errors.New(errors.CategoryTerraform, "no EC2 instances found")

	// ErrHCLParse indicates an error parsing HCL syntax.
	ErrHCLParse = errors.New(errors.CategoryTerraform, "HCL parsing error")
)

// Ensure error types implement the DriftError interface.
var (
	_ errors.DriftError = (*ParseError)(nil)
	_ errors.DriftError = (*ValidationError)(nil)
	_ errors.DriftError = (*ResourceNotFoundError)(nil)
	_ errors.DriftError = (*FileTypeError)(nil)
)
