package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewPool(t *testing.T) {
	t.Run("creates pool with specified concurrency", func(t *testing.T) {
		pool := NewPool(5)

		if pool.Concurrency() != 5 {
			t.Errorf("expected concurrency 5, got %d", pool.Concurrency())
		}
	})

	t.Run("defaults to NumCPU for zero concurrency", func(t *testing.T) {
		pool := NewPool(0)

		if pool.Concurrency() <= 0 {
			t.Error("expected positive concurrency")
		}
	})

	t.Run("defaults to NumCPU for negative concurrency", func(t *testing.T) {
		pool := NewPool(-1)

		if pool.Concurrency() <= 0 {
			t.Error("expected positive concurrency")
		}
	})
}

func TestRun(t *testing.T) {
	t.Run("executes all jobs and returns results in order", func(t *testing.T) {
		pool := NewPool(2)

		jobs := []Job[int, int]{
			{
				Input:   1,
				Execute: func(ctx context.Context, n int) (int, error) { return n * 2, nil },
			},
			{
				Input:   2,
				Execute: func(ctx context.Context, n int) (int, error) { return n * 2, nil },
			},
			{
				Input:   3,
				Execute: func(ctx context.Context, n int) (int, error) { return n * 2, nil },
			},
		}

		results := Run(context.Background(), pool, jobs)

		if len(results) != 3 {
			t.Fatalf("expected 3 results, got %d", len(results))
		}

		// Results should maintain order
		for i, r := range results {
			if r.Err != nil {
				t.Errorf("unexpected error at index %d: %v", i, r.Err)
			}
			expected := (i + 1) * 2
			if r.Value != expected {
				t.Errorf("expected %d at index %d, got %d", expected, i, r.Value)
			}
			if r.Index != i {
				t.Errorf("expected index %d, got %d", i, r.Index)
			}
		}
	})

	t.Run("limits concurrency", func(t *testing.T) {
		pool := NewPool(2)

		var concurrent int32
		var maxConcurrent int32

		jobs := make([]Job[int, struct{}], 10)
		for i := range jobs {
			jobs[i] = Job[int, struct{}]{
				Input: i,
				Execute: func(ctx context.Context, n int) (struct{}, error) {
					current := atomic.AddInt32(&concurrent, 1)
					for {
						max := atomic.LoadInt32(&maxConcurrent)
						if current > max {
							if atomic.CompareAndSwapInt32(&maxConcurrent, max, current) {
								break
							}
						} else {
							break
						}
					}
					time.Sleep(10 * time.Millisecond)
					atomic.AddInt32(&concurrent, -1)
					return struct{}{}, nil
				},
			}
		}

		Run(context.Background(), pool, jobs)

		if atomic.LoadInt32(&maxConcurrent) > 2 {
			t.Errorf("expected max concurrency of 2, got %d", maxConcurrent)
		}
	})

	t.Run("handles empty job list", func(t *testing.T) {
		pool := NewPool(2)

		results := Run[int, int](context.Background(), pool, nil)

		if results != nil {
			t.Error("expected nil results for empty job list")
		}
	})

	t.Run("propagates errors from jobs", func(t *testing.T) {
		pool := NewPool(2)

		expectedErr := errors.New("job error")
		jobs := []Job[int, int]{
			{
				Input:   1,
				Execute: func(ctx context.Context, n int) (int, error) { return 0, expectedErr },
			},
		}

		results := Run(context.Background(), pool, jobs)

		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if !errors.Is(results[0].Err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, results[0].Err)
		}
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		pool := NewPool(2)

		ctx, cancel := context.WithCancel(context.Background())

		jobs := make([]Job[int, int], 10)
		for i := range jobs {
			jobs[i] = Job[int, int]{
				Input: i,
				Execute: func(ctx context.Context, n int) (int, error) {
					time.Sleep(100 * time.Millisecond)
					return n, nil
				},
			}
		}

		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()

		results := Run(ctx, pool, jobs)

		// At least some jobs should have context.Canceled error
		canceledCount := 0
		for _, r := range results {
			if errors.Is(r.Err, context.Canceled) {
				canceledCount++
			}
		}

		if canceledCount == 0 {
			t.Error("expected at least one canceled job")
		}
	})
}

func TestRunFunc(t *testing.T) {
	t.Run("applies function to all inputs", func(t *testing.T) {
		pool := NewPool(2)

		inputs := []string{"a", "bb", "ccc"}
		results := RunFunc(
			context.Background(),
			pool,
			inputs,
			func(ctx context.Context, s string) (int, error) {
				return len(s), nil
			},
		)

		if len(results) != 3 {
			t.Fatalf("expected 3 results, got %d", len(results))
		}

		expected := []int{1, 2, 3}
		for i, r := range results {
			if r.Err != nil {
				t.Errorf("unexpected error at index %d: %v", i, r.Err)
			}
			if r.Value != expected[i] {
				t.Errorf("expected %d at index %d, got %d", expected[i], i, r.Value)
			}
		}
	})
}

func TestMap(t *testing.T) {
	t.Run("returns all successful results", func(t *testing.T) {
		pool := NewPool(2)

		inputs := []int{1, 2, 3}
		values, err := Map(
			context.Background(),
			pool,
			inputs,
			func(ctx context.Context, n int) (int, error) {
				return n * 2, nil
			},
		)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(values) != 3 {
			t.Errorf("expected 3 values, got %d", len(values))
		}
	})

	t.Run("returns error on any failure", func(t *testing.T) {
		pool := NewPool(2)

		inputs := []int{1, 2, 3}
		_, err := Map(
			context.Background(),
			pool,
			inputs,
			func(ctx context.Context, n int) (int, error) {
				if n == 2 {
					return 0, errors.New("error on 2")
				}
				return n * 2, nil
			},
		)

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestForEach(t *testing.T) {
	t.Run("executes for all inputs", func(t *testing.T) {
		pool := NewPool(2)

		var count int32
		inputs := []int{1, 2, 3}
		err := ForEach(context.Background(), pool, inputs, func(ctx context.Context, n int) error {
			atomic.AddInt32(&count, 1)
			return nil
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if atomic.LoadInt32(&count) != 3 {
			t.Errorf("expected count 3, got %d", count)
		}
	})

	t.Run("returns first error encountered", func(t *testing.T) {
		pool := NewPool(1) // Single worker to ensure order

		inputs := []int{1, 2, 3}
		err := ForEach(context.Background(), pool, inputs, func(ctx context.Context, n int) error {
			if n == 2 {
				return errors.New("error on 2")
			}
			return nil
		})

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCollector(t *testing.T) {
	t.Run("Add collects results", func(t *testing.T) {
		collector := NewCollector[int]()

		collector.Add(1)
		collector.Add(2)
		collector.Add(3)

		results := collector.Results()
		if len(results) != 3 {
			t.Errorf("expected 3 results, got %d", len(results))
		}
	})

	t.Run("AddError collects errors", func(t *testing.T) {
		collector := NewCollector[int]()

		collector.AddError(errors.New("error 1"))
		collector.AddError(errors.New("error 2"))

		if !collector.HasErrors() {
			t.Error("expected HasErrors to be true")
		}

		errs := collector.Errors()
		if len(errs) != 2 {
			t.Errorf("expected 2 errors, got %d", len(errs))
		}
	})

	t.Run("is safe for concurrent use", func(t *testing.T) {
		collector := NewCollector[int]()

		done := make(chan struct{})
		for i := range 10 {
			go func(n int) {
				collector.Add(n)
				done <- struct{}{}
			}(i)
		}

		for range 10 {
			<-done
		}

		results := collector.Results()
		if len(results) != 10 {
			t.Errorf("expected 10 results, got %d", len(results))
		}
	})
}

func BenchmarkRun(b *testing.B) {
	pool := NewPool(10)

	jobs := make([]Job[int, int], 100)
	for i := range jobs {
		jobs[i] = Job[int, int]{
			Input: i,
			Execute: func(ctx context.Context, n int) (int, error) {
				return n * 2, nil
			},
		}
	}

	b.ResetTimer()
	for range b.N {
		Run(context.Background(), pool, jobs)
	}
}
