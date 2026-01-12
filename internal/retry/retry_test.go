package retry

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestDo(t *testing.T) {
	t.Run("succeeds on first attempt", func(t *testing.T) {
		cfg := Config{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
			Multiplier:   2.0,
		}

		var attempts int32
		result, err := Do(context.Background(), cfg, func(ctx context.Context) (string, error) {
			atomic.AddInt32(&attempts, 1)
			return "success", nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != "success" {
			t.Errorf("expected 'success', got %q", result)
		}
		if atomic.LoadInt32(&attempts) != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("retries on failure and eventually succeeds", func(t *testing.T) {
		cfg := Config{
			MaxAttempts:  5,
			InitialDelay: 1 * time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   2.0,
		}

		var attempts int32
		result, err := Do(context.Background(), cfg, func(ctx context.Context) (int, error) {
			attempt := atomic.AddInt32(&attempts, 1)
			if attempt < 3 {
				return 0, errors.New("temporary error")
			}
			return 42, nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 42 {
			t.Errorf("expected 42, got %d", result)
		}
		if atomic.LoadInt32(&attempts) != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("fails after max attempts", func(t *testing.T) {
		cfg := Config{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   2.0,
		}

		var attempts int32
		_, err := Do(context.Background(), cfg, func(ctx context.Context) (string, error) {
			atomic.AddInt32(&attempts, 1)
			return "", errors.New("persistent error")
		})

		if err == nil {
			t.Error("expected error after max attempts")
		}
		if atomic.LoadInt32(&attempts) != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		cfg := Config{
			MaxAttempts:  10,
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     1 * time.Second,
			Multiplier:   2.0,
		}

		ctx, cancel := context.WithCancel(context.Background())

		var attempts int32
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		_, err := Do(ctx, cfg, func(ctx context.Context) (string, error) {
			atomic.AddInt32(&attempts, 1)
			return "", errors.New("error")
		})

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error, got %v", err)
		}
	})

	t.Run("stops retrying for non-retryable errors", func(t *testing.T) {
		nonRetryableErr := errors.New("non-retryable")

		cfg := Config{
			MaxAttempts:  5,
			InitialDelay: 1 * time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   2.0,
			ShouldRetry: func(err error) bool {
				return !errors.Is(err, nonRetryableErr)
			},
		}

		var attempts int32
		_, err := Do(context.Background(), cfg, func(ctx context.Context) (string, error) {
			atomic.AddInt32(&attempts, 1)
			return "", nonRetryableErr
		})

		if err == nil {
			t.Error("expected error")
		}
		if atomic.LoadInt32(&attempts) != 1 {
			t.Errorf("expected 1 attempt for non-retryable error, got %d", attempts)
		}
	})

	t.Run("handles MaxAttempts less than 1", func(t *testing.T) {
		cfg := Config{
			MaxAttempts: 0, // Should be treated as 1
		}

		var attempts int32
		_, err := Do(context.Background(), cfg, func(ctx context.Context) (string, error) {
			atomic.AddInt32(&attempts, 1)
			return "", errors.New("error")
		})

		if err == nil {
			t.Error("expected error")
		}
		if atomic.LoadInt32(&attempts) != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})
}

func TestDoSimple(t *testing.T) {
	t.Run("executes operation without return value", func(t *testing.T) {
		cfg := Config{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   2.0,
		}

		executed := false
		err := DoSimple(context.Background(), cfg, func(ctx context.Context) error {
			executed = true
			return nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !executed {
			t.Error("expected operation to be executed")
		}
	})
}

func TestConfigBuilders(t *testing.T) {
	t.Run("WithMaxAttempts updates config", func(t *testing.T) {
		cfg := DefaultConfig.WithMaxAttempts(5)

		if cfg.MaxAttempts != 5 {
			t.Errorf("expected 5, got %d", cfg.MaxAttempts)
		}
	})

	t.Run("WithInitialDelay updates config", func(t *testing.T) {
		cfg := DefaultConfig.WithInitialDelay(500 * time.Millisecond)

		if cfg.InitialDelay != 500*time.Millisecond {
			t.Errorf("expected 500ms, got %v", cfg.InitialDelay)
		}
	})

	t.Run("WithMaxDelay updates config", func(t *testing.T) {
		cfg := DefaultConfig.WithMaxDelay(30 * time.Second)

		if cfg.MaxDelay != 30*time.Second {
			t.Errorf("expected 30s, got %v", cfg.MaxDelay)
		}
	})

	t.Run("WithMultiplier updates config", func(t *testing.T) {
		cfg := DefaultConfig.WithMultiplier(3.0)

		if cfg.Multiplier != 3.0 {
			t.Errorf("expected 3.0, got %f", cfg.Multiplier)
		}
	})

	t.Run("WithJitter updates config", func(t *testing.T) {
		cfg := DefaultConfig.WithJitter(0.5)

		if cfg.Jitter != 0.5 {
			t.Errorf("expected 0.5, got %f", cfg.Jitter)
		}
	})

	t.Run("WithShouldRetry updates config", func(t *testing.T) {
		called := false
		shouldRetry := func(err error) bool {
			called = true
			return false
		}

		cfg := DefaultConfig.WithShouldRetry(shouldRetry)
		cfg.ShouldRetry(errors.New("test"))

		if !called {
			t.Error("expected ShouldRetry to be called")
		}
	})
}

func TestCalculateDelay(t *testing.T) {
	t.Run("increases exponentially", func(t *testing.T) {
		cfg := Config{
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     10 * time.Second,
			Multiplier:   2.0,
			Jitter:       0, // No jitter for predictable test
		}

		delay0 := calculateDelay(cfg, 0)
		delay1 := calculateDelay(cfg, 1)
		delay2 := calculateDelay(cfg, 2)

		if delay0 != 100*time.Millisecond {
			t.Errorf("expected 100ms, got %v", delay0)
		}
		if delay1 != 200*time.Millisecond {
			t.Errorf("expected 200ms, got %v", delay1)
		}
		if delay2 != 400*time.Millisecond {
			t.Errorf("expected 400ms, got %v", delay2)
		}
	})

	t.Run("caps at max delay", func(t *testing.T) {
		cfg := Config{
			InitialDelay: 1 * time.Second,
			MaxDelay:     5 * time.Second,
			Multiplier:   10.0,
			Jitter:       0,
		}

		delay := calculateDelay(cfg, 5) // Would be 100000 seconds without cap

		if delay != 5*time.Second {
			t.Errorf("expected max delay of 5s, got %v", delay)
		}
	})

	t.Run("applies jitter within bounds", func(t *testing.T) {
		cfg := Config{
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     10 * time.Second,
			Multiplier:   2.0,
			Jitter:       0.5, // 50% jitter
		}

		// Run multiple times to test jitter randomness
		for i := 0; i < 10; i++ {
			delay := calculateDelay(cfg, 0)
			// With 50% jitter, delay should be between 50ms and 150ms
			if delay < 50*time.Millisecond || delay > 150*time.Millisecond {
				t.Errorf("delay %v outside expected jitter range", delay)
			}
		}
	})
}

func TestDoWithCallback(t *testing.T) {
	t.Run("calls callback after each attempt", func(t *testing.T) {
		cfg := Config{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   2.0,
		}

		var attempts []Attempt
		var attemptCount int32

		_, _ = DoWithCallback(context.Background(), cfg,
			func(ctx context.Context) (string, error) {
				attempt := atomic.AddInt32(&attemptCount, 1)
				if attempt < 3 {
					return "", errors.New("error")
				}
				return "success", nil
			},
			func(a Attempt) {
				attempts = append(attempts, a)
			},
		)

		if len(attempts) != 3 {
			t.Errorf("expected 3 callback calls, got %d", len(attempts))
		}

		// Check attempt numbers
		for i, a := range attempts {
			if a.Number != i+1 {
				t.Errorf("expected attempt number %d, got %d", i+1, a.Number)
			}
		}

		// Last attempt should have no delay (success)
		if attempts[2].Delay != 0 {
			t.Errorf("expected no delay for successful attempt, got %v", attempts[2].Delay)
		}

		// Failed attempts should have delay
		if attempts[0].Delay == 0 {
			t.Error("expected non-zero delay for failed attempt")
		}
	})
}

func TestDefaultConfigs(t *testing.T) {
	t.Run("DefaultConfig has sensible values", func(t *testing.T) {
		if DefaultConfig.MaxAttempts < 1 {
			t.Error("MaxAttempts should be at least 1")
		}
		if DefaultConfig.InitialDelay <= 0 {
			t.Error("InitialDelay should be positive")
		}
		if DefaultConfig.MaxDelay <= DefaultConfig.InitialDelay {
			t.Error("MaxDelay should be greater than InitialDelay")
		}
		if DefaultConfig.Multiplier <= 1 {
			t.Error("Multiplier should be greater than 1")
		}
	})

	t.Run("AWSConfig has sensible values", func(t *testing.T) {
		if AWSConfig.MaxAttempts < 1 {
			t.Error("MaxAttempts should be at least 1")
		}
		if AWSConfig.InitialDelay <= 0 {
			t.Error("InitialDelay should be positive")
		}
	})

	t.Run("FastConfig has shorter delays", func(t *testing.T) {
		if FastConfig.InitialDelay >= DefaultConfig.InitialDelay {
			t.Error("FastConfig should have shorter InitialDelay")
		}
		if FastConfig.MaxDelay >= DefaultConfig.MaxDelay {
			t.Error("FastConfig should have shorter MaxDelay")
		}
	})
}
