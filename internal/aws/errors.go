// Package aws provides functionality to interact with AWS EC2 service.
package aws

import (
	stderrors "errors"
	"fmt"
	"net"
	"slices"
	"strings"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/smithy-go"

	"github.com/solomon-os/go-test/internal/errors"
	"github.com/solomon-os/go-test/internal/retry"
)

// AWSError represents errors from AWS API operations.
// It provides detailed context about the failed operation including
// the AWS error code and HTTP status code for debugging and retry logic.
type AWSError struct {
	errors.BaseError
	// Operation is the AWS API operation that failed (e.g., "DescribeInstances").
	Operation string
	// InstanceID is the EC2 instance ID if applicable.
	InstanceID string
	// AWSCode is the AWS error code (e.g., "ThrottlingException").
	AWSCode string
	// StatusCode is the HTTP status code from the response.
	StatusCode int
}

// AWSErrorOption is a functional option for configuring an AWSError.
type AWSErrorOption func(*AWSError)

// WithInstanceID sets the instance ID on the error.
func WithInstanceID(id string) AWSErrorOption {
	return func(e *AWSError) {
		e.InstanceID = id
	}
}

// WithAWSCode sets the AWS error code.
func WithAWSCode(code string) AWSErrorOption {
	return func(e *AWSError) {
		e.AWSCode = code
	}
}

// WithStatusCode sets the HTTP status code.
func WithStatusCode(code int) AWSErrorOption {
	return func(e *AWSError) {
		e.StatusCode = code
	}
}

// NewAWSError creates a new AWS error with the given operation and cause.
func NewAWSError(operation string, cause error, opts ...AWSErrorOption) *AWSError {
	e := &AWSError{
		BaseError: errors.BaseError{},
		Operation: operation,
	}
	e.BaseError = *errors.New(errors.CategoryAWS, fmt.Sprintf("%s failed", operation)).
		WithCause(cause)

	// Extract AWS-specific information from the cause
	extractAWSErrorInfo(cause, e)

	// Apply options
	for _, opt := range opts {
		opt(e)
	}

	// Set retryable based on error type
	e.BaseError = *e.WithRetryable(IsRetryableError(cause))

	return e
}

// Error implements the error interface.
func (e *AWSError) Error() string {
	msg := fmt.Sprintf("AWS %s failed", e.Operation)
	if e.InstanceID != "" {
		msg += fmt.Sprintf(" for instance %s", e.InstanceID)
	}
	if e.AWSCode != "" {
		msg += fmt.Sprintf(" [%s]", e.AWSCode)
	}
	if cause := e.Unwrap(); cause != nil {
		msg += fmt.Sprintf(": %v", cause)
	}
	return msg
}

// extractAWSErrorInfo extracts AWS-specific error information from the error.
func extractAWSErrorInfo(err error, awsErr *AWSError) {
	if err == nil {
		return
	}

	// Check for HTTP response error
	var respErr *awshttp.ResponseError
	if stderrors.As(err, &respErr) {
		awsErr.StatusCode = respErr.HTTPStatusCode()
	}

	// Check for API error with code
	var apiErr smithy.APIError
	if stderrors.As(err, &apiErr) {
		awsErr.AWSCode = apiErr.ErrorCode()
	}
}

// IsRetryableError determines if an AWS error should be retried.
// It checks for rate limiting, transient server errors, and network issues.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for HTTP response errors
	var respErr *awshttp.ResponseError
	if stderrors.As(err, &respErr) {
		switch respErr.HTTPStatusCode() {
		case 429: // Too Many Requests
			return true
		case 500, 502, 503, 504: // Server errors
			return true
		}
	}

	// Check for specific AWS error codes
	var apiErr smithy.APIError
	if stderrors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		retryableCodes := []string{
			"ThrottlingException",
			"Throttling",
			"RequestLimitExceeded",
			"ProvisionedThroughputExceededException",
			"ServiceUnavailable",
			"ServiceUnavailableException",
			"InternalError",
			"InternalServiceError",
			"RequestTimeout",
			"RequestTimeoutException",
		}
		if slices.Contains(retryableCodes, code) {
			return true
		}
	}

	// Check for network errors (timeout only - Temporary is deprecated)
	var netErr net.Error
	if stderrors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Check for connection reset or timeout in error message
	errMsg := err.Error()
	retryablePatterns := []string{
		"connection reset",
		"connection refused",
		"i/o timeout",
		"no such host",
		"TLS handshake timeout",
		"context deadline exceeded",
	}
	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errMsg), strings.ToLower(pattern)) {
			return true
		}
	}

	// Check if the error implements our retryable interface
	return errors.IsRetryable(err)
}

// Sentinel errors for common AWS conditions.
var (
	// ErrInstanceNotFound indicates the EC2 instance was not found.
	ErrInstanceNotFound = errors.New(errors.CategoryAWS, "instance not found")

	// ErrRateLimited indicates the request was rate limited by AWS.
	ErrRateLimited = errors.New(errors.CategoryAWS, "rate limited").WithRetryable(true)

	// ErrAccessDenied indicates access was denied to the AWS resource.
	ErrAccessDenied = errors.New(errors.CategoryAWS, "access denied")

	// ErrInvalidCredentials indicates AWS credentials are invalid or expired.
	ErrInvalidCredentials = errors.New(errors.CategoryAWS, "invalid credentials")

	// ErrRegionNotFound indicates the specified AWS region is invalid.
	ErrRegionNotFound = errors.New(errors.CategoryAWS, "region not found")
)

// IsNotFoundError checks if the error indicates a resource was not found.
func IsNotFoundError(err error) bool {
	if errors.Is(err, ErrInstanceNotFound) {
		return true
	}

	var apiErr smithy.APIError
	if stderrors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		return code == "InvalidInstanceID.NotFound" ||
			code == "InvalidInstanceID.Malformed" ||
			strings.Contains(code, "NotFound")
	}

	return false
}

// IsAccessDeniedError checks if the error indicates access was denied.
func IsAccessDeniedError(err error) bool {
	if errors.Is(err, ErrAccessDenied) {
		return true
	}

	var apiErr smithy.APIError
	if stderrors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		return code == "UnauthorizedOperation" ||
			code == "AccessDenied" ||
			code == "AccessDeniedException" ||
			strings.HasPrefix(code, "AccessDenied")
	}

	return false
}

// Ensure AWSError implements the DriftError interface.
var _ errors.DriftError = (*AWSError)(nil)

// ClientOption is a functional option for configuring the AWS client.
type ClientOption func(*clientOptions)

type clientOptions struct {
	retryConfig retry.Config
}

// WithRetryConfig sets the retry configuration for the client.
func WithRetryConfig(cfg retry.Config) ClientOption {
	return func(o *clientOptions) {
		o.retryConfig = cfg
	}
}
