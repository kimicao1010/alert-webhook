package dingtalk

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
)

// Test_Sign_MatchesDingtalk 验证签名算法与钉钉服务器端实际校验的算法一致。
// 实测（2026-08）：钉钉接受 HMAC-SHA256(key=secret, data=timestamp+"\n"+secret)；
// 官方文档 Python 示例（hmac.new(string_to_sign, digestmod=sha256)）因 Python
// hmac.new 首个参数是 key 而实际签名不符，返回 errcode 310000。
// 本测试用独立的 Python 语义复算（key=secret, msg=timestamp\nsecret）比对。
func Test_Sign_MatchesDingtalk(t *testing.T) {
	secret := "SEC1234567890abcdef"
	timestamp := "1733123456789"

	// 独立复算：HMAC(key=secret, msg=timestamp\nsecret)
	stringToSign := timestamp + "\n" + secret
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	expected := url.QueryEscape(base64.StdEncoding.EncodeToString(h.Sum(nil)))

	got := sign(secret, timestamp)
	if got != expected {
		t.Fatalf("sign mismatch:\n got:      %s\n expected: %s", got, expected)
	}
	if got == "" || strings.Contains(got, "\n") {
		t.Fatalf("sign should be non-empty urlencoded base64, got: %q", got)
	}
}

// Test_Sign_DiffersFromDocPythonExample 验证我们的签名确实不同于文档 Python 示例的
// 实际行为（key=timestamp\nsecret, data=""），避免误用错误算法。
func Test_Sign_DiffersFromDocPythonExample(t *testing.T) {
	secret := "SEC1234567890abcdef"
	timestamp := "1733123456789"

	// 文档 Python 示例实际行为：hmac.new(string_to_sign, digestmod=sha256) → key=string_to_sign, data=""
	stringToSign := timestamp + "\n" + secret
	h := hmac.New(sha256.New, []byte(stringToSign))
	docExample := url.QueryEscape(base64.StdEncoding.EncodeToString(h.Sum(nil)))

	if got := sign(secret, timestamp); got == docExample {
		t.Fatalf("sign should differ from doc Python example (which dingtalk rejects): %s", got)
	}
}

// Test_Addr_WithSecret 验证配置 secret 时 Addr 包含 timestamp 与 sign 参数。
func Test_Addr_WithSecret(t *testing.T) {
	bot := NewDingtalkGroupBotWithSecret("tok123", "SECxxx")
	addr := bot.Addr()
	if !strings.Contains(addr, "access_token=tok123") {
		t.Fatalf("missing access_token: %s", addr)
	}
	if !strings.Contains(addr, "timestamp=") {
		t.Fatalf("missing timestamp: %s", addr)
	}
	if !strings.Contains(addr, "sign=") {
		t.Fatalf("missing sign: %s", addr)
	}
}

// Test_Addr_WithoutSecret 验证未配置 secret 时行为不变（无签名参数，兼容旧配置）。
func Test_Addr_WithoutSecret(t *testing.T) {
	bot := NewDingtalkGroupBot("tok123")
	addr := bot.Addr()
	if strings.Contains(addr, "timestamp=") || strings.Contains(addr, "sign=") {
		t.Fatalf("addr should not contain sign params without secret: %s", addr)
	}
}
