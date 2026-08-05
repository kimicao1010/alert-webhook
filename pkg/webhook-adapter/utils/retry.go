package utils

import (
	"fmt"
	"time"
)

// RetryLogFunc 在每次重试前被调用，用于记录重试信息。
// attempt 是刚失败的尝试序号（从 1 开始），backoff 是即将等待的退避时长。
type RetryLogFunc func(attempt int, err error, backoff time.Duration)

// Retry 执行 fn，失败时按固定间隔序列退避重试。
// attempts 为总尝试次数（含首次），backoffs 为每次重试前的等待时长序列
// （长度通常为 attempts-1；不足时按最后一个值补齐，超出时截断）。
// fn 返回 nil 立即成功返回；耗尽尝试次数后返回包装了最后一次错误的 error。
func Retry(attempts int, backoffs []time.Duration, onRetry RetryLogFunc, fn func() error) error {
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
		backoff := fixedBackoff(attempt-1, backoffs)
		if onRetry != nil {
			onRetry(attempt, lastErr, backoff)
		}
		time.Sleep(backoff)
	}
	return fmt.Errorf("failed after %d attempts, last err: %w", attempts, lastErr)
}

// fixedBackoff 取固定间隔序列中的第 i 个值；i 越界时取最后一个，空序列返回 0。
func fixedBackoff(i int, backoffs []time.Duration) time.Duration {
	if len(backoffs) == 0 {
		return 0
	}
	if i >= len(backoffs) {
		return backoffs[len(backoffs)-1]
	}
	return backoffs[i]
}
