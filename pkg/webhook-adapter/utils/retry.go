package utils

import (
	"fmt"
	"time"
)

// RetryLogFunc 在每次重试前被调用，用于记录重试信息。
// attempt 是刚失败的尝试序号（从 1 开始），backoff 是即将等待的退避时长。
type RetryLogFunc func(attempt int, err error, backoff time.Duration)

// Retry 执行 fn，失败时按指数退避重试。
// attempts 为总尝试次数（含首次），baseBackoff 为第一次重试前的等待时长，
// 之后每次翻倍（baseBackoff, 2x, 4x, ...）。
// fn 返回 nil 立即成功返回；耗尽尝试次数后返回包装了最后一次错误的 error。
func Retry(attempts int, baseBackoff time.Duration, onRetry RetryLogFunc, fn func() error) error {
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if attempt == attempts {
			break
		}
		backoff := baseBackoff * (1 << (attempt - 1))
		if onRetry != nil {
			onRetry(attempt, lastErr, backoff)
		}
		time.Sleep(backoff)
	}
	return fmt.Errorf("failed after %d attempts, last err: %w", attempts, lastErr)
}
