// Package worker provides concurrent work execution patterns with bounded concurrency.
//
// This package implements a worker pool pattern that limits the number of
// concurrent goroutines processing work items. This prevents resource exhaustion
// when processing large numbers of items and provides better control over
// system resource usage.
//
// Example usage:
//
//	pool := worker.NewPool(10) // max 10 concurrent workers
//
//	jobs := []worker.Job[string, int]{
//	    {Input: "hello", Execute: func(ctx context.Context, s string) (int, error) {
//	        return len(s), nil
//	    }},
//	}
//
//	results := worker.Run(ctx, pool, jobs)
//	for _, r := range results {
//	    if r.Err != nil {
//	        log.Printf("error: %v", r.Err)
//	    } else {
//	        log.Printf("result: %d", r.Value)
//	    }
//	}
package worker

import (
	"context"
	"runtime"
	"sync"

	"github.com/solomon-os/go-test/internal/logger"
)

// Pool manages a bounded set of concurrent workers.
// It uses a semaphore pattern to limit the number of goroutines
// that can execute work simultaneously.
type Pool struct {
	concurrency int
	sem         chan struct{}
}

// NewPool creates a worker pool with the specified concurrency limit.
// If concurrency is <= 0, it defaults to the number of CPUs.
func NewPool(concurrency int) *Pool {
	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
	}
	return &Pool{
		concurrency: concurrency,
		sem:         make(chan struct{}, concurrency),
	}
}

// Concurrency returns the maximum number of concurrent workers.
func (p *Pool) Concurrency() int {
	return p.concurrency
}

// Job represents a unit of work with typed input and output.
type Job[T any, R any] struct {
	// Input is the data to be processed.
	Input T
	// Execute is the function that processes the input and returns a result.
	Execute func(context.Context, T) (R, error)
}

// Result wraps job output with potential error.
type Result[R any] struct {
	// Value is the result of the job execution.
	Value R
	// Err is any error that occurred during execution.
	Err error
	// Index is the original position of this job in the input slice.
	Index int
}

// Run executes jobs with bounded concurrency, maintaining result order.
// Jobs are executed concurrently but results are returned in the same order
// as the input jobs slice.
func Run[T, R any](ctx context.Context, pool *Pool, jobs []Job[T, R]) []Result[R] {
	if len(jobs) == 0 {
		return nil
	}

	logger.Debug("starting worker pool execution",
		"job_count", len(jobs),
		"concurrency", pool.concurrency)

	results := make([]Result[R], len(jobs))
	var wg sync.WaitGroup

	for i, job := range jobs {
		wg.Add(1)
		go func(idx int, j Job[T, R]) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case pool.sem <- struct{}{}:
				defer func() { <-pool.sem }() // Release on exit
			case <-ctx.Done():
				results[idx] = Result[R]{
					Err:   ctx.Err(),
					Index: idx,
				}
				return
			}

			// Check context again after acquiring semaphore
			select {
			case <-ctx.Done():
				results[idx] = Result[R]{
					Err:   ctx.Err(),
					Index: idx,
				}
				return
			default:
			}

			// Execute the job
			value, err := j.Execute(ctx, j.Input)
			results[idx] = Result[R]{
				Value: value,
				Err:   err,
				Index: idx,
			}
		}(i, job)
	}

	wg.Wait()

	logger.Debug("worker pool execution completed", "job_count", len(jobs))

	return results
}

// RunFunc is a convenience function that runs a single function over multiple inputs.
// It's useful when you have the same operation to apply to many items.
func RunFunc[T, R any](
	ctx context.Context,
	pool *Pool,
	inputs []T,
	fn func(context.Context, T) (R, error),
) []Result[R] {
	jobs := make([]Job[T, R], len(inputs))
	for i, input := range inputs {
		jobs[i] = Job[T, R]{
			Input:   input,
			Execute: fn,
		}
	}
	return Run(ctx, pool, jobs)
}

// Map applies a function to each input and collects successful results.
// Unlike Run, Map returns only the successful results and an aggregated error
// for any failures. The order of successful results may not match input order.
func Map[T, R any](
	ctx context.Context,
	pool *Pool,
	inputs []T,
	fn func(context.Context, T) (R, error),
) ([]R, error) {
	results := RunFunc(ctx, pool, inputs, fn)

	var (
		values []R
		errs   []error
	)

	for _, r := range results {
		if r.Err != nil {
			errs = append(errs, r.Err)
		} else {
			values = append(values, r.Value)
		}
	}

	if len(errs) > 0 {
		// Return first error for simplicity; could return AggregateError
		return values, errs[0]
	}

	return values, nil
}

// ForEach applies a function to each input without collecting results.
// This is useful for side-effecting operations.
func ForEach[T any](
	ctx context.Context,
	pool *Pool,
	inputs []T,
	fn func(context.Context, T) error,
) error {
	results := RunFunc(ctx, pool, inputs, func(ctx context.Context, input T) (struct{}, error) {
		return struct{}{}, fn(ctx, input)
	})

	for _, r := range results {
		if r.Err != nil {
			return r.Err
		}
	}

	return nil
}

// Collector accumulates results from concurrent operations.
// It is safe for concurrent use.
type Collector[R any] struct {
	mu      sync.Mutex
	results []R
	errors  []error
}

// NewCollector creates a new result collector.
func NewCollector[R any]() *Collector[R] {
	return &Collector[R]{
		results: make([]R, 0),
		errors:  make([]error, 0),
	}
}

// Add appends a result to the collector.
func (c *Collector[R]) Add(result R) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results = append(c.results, result)
}

// AddError appends an error to the collector.
func (c *Collector[R]) AddError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errors = append(c.errors, err)
}

// Results returns all collected results.
func (c *Collector[R]) Results() []R {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.results
}

// Errors returns all collected errors.
func (c *Collector[R]) Errors() []error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.errors
}

// HasErrors returns true if any errors were collected.
func (c *Collector[R]) HasErrors() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.errors) > 0
}
