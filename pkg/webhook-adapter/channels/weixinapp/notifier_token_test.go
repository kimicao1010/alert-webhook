package weixinapp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// Test_Notifier_TokenRefresh 验证 token 过期后 Send 会自动重新获取 token（T004 修复点）。
// 使用 httptest 模拟企业微信接口，不依赖真实凭据。
func Test_Notifier_TokenRefresh(t *testing.T) {
	var getTokenCount int64
	var issuedTokens int64

	mux := http.NewServeMux()
	mux.HandleFunc("/cgi-bin/gettoken", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&getTokenCount, 1)
		n := atomic.AddInt64(&issuedTokens, 1)
		resp := map[string]any{
			"errcode":      0,
			"errmsg":       "ok",
			"access_token": "mock-token-" + string(rune('0'+n)),
			"expires_in":   7200,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/cgi-bin/message/send", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("access_token") == "" {
			t.Errorf("message/send called without access_token")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	n := &Notifier{
		addr:        srv.URL,
		corpID:      "test-corp",
		agentID:     1,
		agentSecret: "test-secret",
		client:      srv.Client(),
		toUser:      "@all",
	}

	// 第一次发送：token 为空，应触发一次 GetToken
	msg := NewMsgText("hello")
	if err := n.Send(msg); err != nil {
		t.Fatalf("first send failed: %v", err)
	}
	if got := atomic.LoadInt64(&getTokenCount); got != 1 {
		t.Fatalf("expected gettoken called 1 time after first send, got %d", got)
	}

	// token 仍在有效期内：再次发送不应重新获取 token
	msg = NewMsgText("hello again")
	if err := n.Send(msg); err != nil {
		t.Fatalf("second send failed: %v", err)
	}
	if got := atomic.LoadInt64(&getTokenCount); got != 1 {
		t.Fatalf("expected gettoken still 1 time when token valid, got %d", got)
	}

	// 模拟 token 过期（tokenAt 回拨超过有效期）
	n.mu.Lock()
	n.tokenAt = n.tokenAt.Add(-(n.tokenExpiredIn + time.Minute))
	n.mu.Unlock()

	// 过期后发送：应自动重新获取 token（T004 修复的核心行为）
	msg = NewMsgText("hello after expiry")
	if err := n.Send(msg); err != nil {
		t.Fatalf("send after token expiry failed: %v", err)
	}
	if got := atomic.LoadInt64(&getTokenCount); got != 2 {
		t.Fatalf("expected gettoken called 2 times after token expiry, got %d", got)
	}
}

// Test_Notifier_ShouldGetToken 验证过期判断逻辑本身。
func Test_Notifier_ShouldGetToken(t *testing.T) {
	n := &Notifier{}

	// 无 token
	if !n.ShouldGetToken() {
		t.Fatal("ShouldGetToken should be true when token is empty")
	}

	// 有效 token
	n.token = "abc"
	n.tokenAt = time.Now()
	n.tokenExpiredIn = 2 * time.Hour
	if n.ShouldGetToken() {
		t.Fatal("ShouldGetToken should be false when token is fresh")
	}

	// 过期 token
	n.tokenAt = time.Now().Add(-3 * time.Hour)
	if !n.ShouldGetToken() {
		t.Fatal("ShouldGetToken should be true when token expired")
	}
}
