package feishu

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Test_Send_ErrCodeValidation 验证 T006：feishu Send 解析响应体 code，业务错误视为失败。
func Test_Send_ErrCodeValidation(t *testing.T) {
	tests := []struct {
		name      string
		respBody  string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "success",
			respBody: `{"code":0,"msg":"success"}`,
			wantErr:  false,
		},
		{
			name:      "business error - invalid token",
			respBody:  `{"code":19001,"msg":"token invalid"}`,
			wantErr:   true,
			errSubstr: "token invalid",
		},
		{
			name:      "business error - rate limit",
			respBody:  `{"code":9499,"msg":"request too fast"}`,
			wantErr:   true,
			errSubstr: "request too fast",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.respBody))
			}))
			defer srv.Close()

			bot := &FeishuGroupBot{
				addr:   srv.URL,
				token:  "test-token",
				client: srv.Client(),
			}

			msg := NewMsgText("hello")
			err := bot.Send(msg)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
