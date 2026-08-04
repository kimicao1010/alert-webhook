package dingtalk

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Test_Send_ErrCodeValidation 验证 T006：dingtalk Send 解析响应体 errcode，业务错误视为失败。
func Test_Send_ErrCodeValidation(t *testing.T) {
	tests := []struct {
		name      string
		respBody  string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "success",
			respBody: `{"errcode":0,"errmsg":"ok"}`,
			wantErr:  false,
		},
		{
			name:      "business error - invalid token",
			respBody:  `{"errcode":310000,"errmsg":"keywords not in content"}`,
			wantErr:   true,
			errSubstr: "keywords not in content",
		},
		{
			name:      "business error - rate limit",
			respBody:  `{"errcode":130101,"errmsg":"send too fast"}`,
			wantErr:   true,
			errSubstr: "send too fast",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.respBody))
			}))
			defer srv.Close()

			bot := &DingtalkGroupBot{
				addr:         srv.URL,
				access_token: "test-token",
				client:       srv.Client(),
			}

			msg := NewMsgText(NewText("hello"))
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
