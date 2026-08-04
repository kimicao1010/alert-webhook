package weixinapp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test_Notifier_Send_ErrCodeValidation 验证 Send() 对企业微信响应体 errcode 的校验（T026 修复点）。
// 企业微信语义：HTTP 200 + errcode≠0 表示业务失败，必须返回 error，不得误报成功。
func Test_Notifier_Send_ErrCodeValidation(t *testing.T) {
	t.Run("errcode非0返回error", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/cgi-bin/gettoken", func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errcode": 0, "errmsg": "ok", "access_token": "mock-token", "expires_in": 7200,
			})
		})
		mux.HandleFunc("/cgi-bin/message/send", func(w http.ResponseWriter, r *http.Request) {
			// 业务失败：HTTP 200 但 errcode=93000
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errcode":93000,"errmsg":"invalid request"}`))
		})

		srv := httptest.NewServer(mux)
		defer srv.Close()

		n := &Notifier{
			addr: srv.URL, corpID: "c", agentID: 1, agentSecret: "s",
			client: srv.Client(), toUser: "@all",
		}

		err := n.Send(NewMsgText("hello"))
		if err == nil {
			t.Fatal("expected error for errcode!=0, got nil")
		}
	})

	t.Run("errcode为0返回成功", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/cgi-bin/gettoken", func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errcode": 0, "errmsg": "ok", "access_token": "mock-token", "expires_in": 7200,
			})
		})
		mux.HandleFunc("/cgi-bin/message/send", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		})

		srv := httptest.NewServer(mux)
		defer srv.Close()

		n := &Notifier{
			addr: srv.URL, corpID: "c", agentID: 1, agentSecret: "s",
			client: srv.Client(), toUser: "@all",
		}

		if err := n.Send(NewMsgText("hello")); err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
	})
}
