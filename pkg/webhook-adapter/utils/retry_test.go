package utils

import (
	"errors"
	"testing"
	"time"
)

// Test_Retry 验证指数退避重试逻辑（T007）。
func Test_Retry(t *testing.T) {
	t.Run("success on first attempt", func(t *testing.T) {
		calls := 0
		err := Retry(4, time.Millisecond, nil, func() error {
			calls++
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls != 1 {
			t.Fatalf("expected 1 call, got %d", calls)
		}
	})

	t.Run("success on third attempt", func(t *testing.T) {
		calls := 0
		var backoffs []time.Duration
		err := Retry(4, time.Millisecond, func(attempt int, e error, backoff time.Duration) {
			backoffs = append(backoffs, backoff)
		}, func() error {
			calls++
			if calls < 3 {
				return errors.New("transient")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls != 3 {
			t.Fatalf("expected 3 calls, got %d", calls)
		}
		if len(backoffs) != 2 {
			t.Fatalf("expected 2 retries logged, got %d", len(backoffs))
		}
		// 退避应指数翻倍：1ms, 2ms
		if backoffs[0] != time.Millisecond || backoffs[1] != 2*time.Millisecond {
			t.Fatalf("unexpected backoffs: %v", backoffs)
		}
	})

	t.Run("all attempts fail", func(t *testing.T) {
		calls := 0
		var backoffs []time.Duration
		err := Retry(4, time.Millisecond, func(attempt int, e error, backoff time.Duration) {
			backoffs = append(backoffs, backoff)
		}, func() error {
			calls++
			return errors.New("permanent")
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if calls != 4 {
			t.Fatalf("expected 4 calls, got %d", calls)
		}
		// 3 次重试退避：1ms, 2ms, 4ms
		if len(backoffs) != 3 {
			t.Fatalf("expected 3 retries logged, got %d", len(backoffs))
		}
		want := []time.Duration{time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond}
		for i, w := range want {
			if backoffs[i] != w {
				t.Fatalf("backoff[%d] = %v, want %v", i, backoffs[i], w)
			}
		}
	})

	t.Run("zero attempts treated as 1", func(t *testing.T) {
		calls := 0
		err := Retry(0, time.Millisecond, nil, func() error {
			calls++
			return errors.New("fail")
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if calls != 1 {
			t.Fatalf("expected 1 call, got %d", calls)
		}
	})
}
