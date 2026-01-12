// Package retry provides retry logic with exponential backoff for handling transient failures.
//
// This package implements a generic retry mechanism that can be applied to any
// operation that may fail transiently. It supports:
//   - Exponential backoff with configurable parameters
//   - Jitter to prevent thundering herd problems
//   - Context cancellation for graceful shutdown
//   - Custom retry conditions via predicate functions
//
// Example usage:
//
//	result, err := retry.Do(ctx, retry.DefaultConfig, func(ctx context.Context) (string, error) {
//	    return fetchData(ctx)
//	})
//
// For AWS-specific retries, use the pre-configured AWSConfig:
//
//	result, err := retry.Do(ctx, retry.AWSConfig, func(ctx context.Context) (*Instance, error) {
//	    return awsClient.GetInstance(ctx, instanceID)
//	})
package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/solomon-os/go-test/internal/errors"
	"github.com/solomon-os/go-test/internal/logger"
)

// Config defines retry behavior.
type Config struct {
	// MaxAttempts is the maximum number of attempts (including the first one).
	// Must be at least 1.
	MaxAttempts int

	// InitialDelay is the delay before the first retry.
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration

	// Multiplier is the factor by which the delay increases after each attempt.
	Multiplier float64

	// Jitter adds randomness to delays to prevent thundering herd.
	// Value between 0.0 (no jitter) and 1.0 (up to 100% jitter).
	Jitter float64

	// ShouldRetry determines if an error should trigger a retry.
	// If nil, all errors are considered retryable.
	ShouldRetry func(error) bool
}

// DefaultConfig provides sensible defaults for general operations.
var DefaultConfig = Config{
	MaxAttempts:  3,
	InitialDelay: 100 * time.Millisecond,
	MaxDelay:     10 * time.Second,
	Multiplier:   2.0,
	Jitter:       0.2,
	ShouldRetry:  nil, // retry all errors
}

// AWSConfig provides retry settings optimized for AWS API calls.
// Uses longer delays and more attempts to handle rate limiting.
var AWSConfig = Config{
	MaxAttempts:  3,
	InitialDelay: 200 * time.Millisecond,
	MaxDelay:     30 * time.Second,
	Multiplier:   2.0,
	Jitter:       0.25,
	ShouldRetry:  nil, // set by aws package
}

// FastConfig provides quick retries for local operations.
var FastConfig = Config{
	MaxAttempts:  3,
	InitialDelay: 10 * time.Millisecond,
	MaxDelay:     100 * time.Millisecond,
	Multiplier:   2.0,
	Jitter:       0.1,
	ShouldRetry:  nil,
}

// Do executes an operation with retry logic.
// The operation is retried according to the config until it succeeds,
// the maximum attempts are reached, or the context is canceled.
func Do[T any](ctx context.Context, cfg Config, operation func(context.Context) (T, error)) (T, error) {
	var zero T
	var lastErr error

	if cfg.MaxAttempts < 1 {
		cfg.MaxAttempts = 1
	}

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return zero, errors.Wrap(ctx.Err(), errors.CategoryInternal,
					fmt.Sprintf("context canceled after %d attempts", attempt))
			}
			return zero, ctx.Err()
		default:
		}

		result, err := operation(ctx)
		if err == nil {
			if attempt > 0 {
				logger.Debug("operation succeeded after retry",
					"attempt", attempt+1,
					"total_attempts", cfg.MaxAttempts)
			}
			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if cfg.ShouldRetry != nil && !cfg.ShouldRetry(err) {
			logger.Debug("error not retryable, stopping",
				"attempt", attempt+1,
				"error", err)
			return zero, err
		}

		// Also check if error implements IsRetryable
		if !isRetryable(err, cfg.ShouldRetry) {
			return zero, err
		}

		// Don't sleep after the last attempt
		if attempt == cfg.MaxAttempts-1 {
			break
		}

		delay := calculateDelay(cfg, attempt)
		logger.Debug("retrying operation",
			"attempt", attempt+1,
			"max_attempts", cfg.MaxAttempts,
			"delay", delay,
			"error", err)

		select {
		case <-ctx.Done():
			return zero, errors.Wrap(ctx.Err(), errors.CategoryInternal,
				fmt.Sprintf("context canceled during retry backoff (attempt %d)", attempt+1))
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return zero, errors.Wrapf(lastErr, errors.CategoryInternal,
		"operation failed after %d attempts", cfg.MaxAttempts).WithRetryable(false)
}

// DoSimple executes an operation that doesn't return a value.
func DoSimple(ctx context.Context, cfg Config, operation func(context.Context) error) error {
	_, err := Do(ctx, cfg, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, operation(ctx)
	})
	return err
}

// WithMaxAttempts returns a copy of the config with updated MaxAttempts.
func (c Config) WithMaxAttempts(n int) Config {
	c.MaxAttempts = n
	return c
}

// WithInitialDelay returns a copy of the config with updated InitialDelay.
func (c Config) WithInitialDelay(d time.Duration) Config {
	c.InitialDelay = d
	return c
}

// WithMaxDelay returns a copy of the config with updated MaxDelay.
func (c Config) WithMaxDelay(d time.Duration) Config {
	c.MaxDelay = d
	return c
}

// WithMultiplier returns a copy of the config with updated Multiplier.
func (c Config) WithMultiplier(m float64) Config {
	c.Multiplier = m
	return c
}

// WithJitter returns a copy of the config with updated Jitter.
func (c Config) WithJitter(j float64) Config {
	c.Jitter = j
	return c
}

// WithShouldRetry returns a copy of the config with a custom retry predicate.
func (c Config) WithShouldRetry(fn func(error) bool) Config {
	c.ShouldRetry = fn
	return c
}

// calculateDelay computes the delay for a given attempt using exponential backoff with jitter.
func calculateDelay(cfg Config, attempt int) time.Duration {
	// Calculate exponential delay
	delay := float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt))

	// Cap at max delay
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	// Apply jitter: add random value between -jitter*delay and +jitter*delay
	if cfg.Jitter > 0 {
		jitterRange := delay * cfg.Jitter
		jitter := (rand.Float64()*2 - 1) * jitterRange // -1 to +1, scaled
		delay += jitter
	}

	// Ensure delay is non-negative
	if delay < 0 {
		delay = 0
	}

	return time.Duration(delay)
}

// isRetryable checks if an error should be retried.
func isRetryable(err error, shouldRetry func(error) bool) bool {
	// If custom predicate is provided, use it
	if shouldRetry != nil {
		return shouldRetry(err)
	}

	// Check if error implements the retryable interface
	if errors.IsRetryable(err) {
		return true
	}

	// Default: all errors are retryable if no predicate is set
	return true
}

// Attempt represents a single retry attempt result.
type Attempt struct {
	Number int           // Attempt number (1-indexed)
	Error  error         // Error from this attempt, nil if successful
	Delay  time.Duration // Delay before next attempt (0 if last attempt)
}

// DoWithCallback executes an operation with retry logic and calls a callback after each attempt.
// This is useful for logging, metrics, or custom backoff strategies.
func DoWithCallback[T any](
	ctx context.Context,
	cfg Config,
	operation func(context.Context) (T, error),
	callback func(Attempt),
) (T, error) {
	var zero T
	var lastErr error

	if cfg.MaxAttempts < 1 {
		cfg.MaxAttempts = 1
	}

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		result, err := operation(ctx)

		// Calculate delay for callback (0 if this is the last attempt or success)
		var delay time.Duration
		if err != nil && attempt < cfg.MaxAttempts-1 {
			delay = calculateDelay(cfg, attempt)
		}

		if callback != nil {
			callback(Attempt{
				Number: attempt + 1,
				Error:  err,
				Delay:  delay,
			})
		}

		if err == nil {
			return result, nil
		}

		lastErr = err

		if !isRetryable(err, cfg.ShouldRetry) {
			return zero, err
		}

		if attempt == cfg.MaxAttempts-1 {
			break
		}

		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(delay):
		}
	}

	return zero, errors.Wrapf(lastErr, errors.CategoryInternal,
		"operation failed after %d attempts", cfg.MaxAttempts)
}
