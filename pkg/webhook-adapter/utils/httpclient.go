package utils

import (
	"net/http"
	"time"
)

// DefaultTimeout 是对外 HTTP 请求的默认超时时间。
const DefaultTimeout = 10 * time.Second

// NewHTTPClient 创建带超时与连接池配置的 http.Client。
// timeout <= 0 时使用 DefaultTimeout。
func NewHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

// SharedClient 是全局共享的 http.Client（默认 10s 超时，连接池复用）。
// http.Client 本身并发安全，各渠道出站请求应复用该实例，避免连接池碎片化。
var SharedClient = NewHTTPClient(DefaultTimeout)
