package weixinapp

import (
	"os"
	"strconv"
	"testing"
)

// Test_Notifier_Send 为集成测试：依赖真实企业微信应用凭据
// （环境变量 WEIXIN_APP_CORP_ID / WEIXIN_APP_AGENT_ID / WEIXIN_APP_SECRET），未配置时跳过。
func Test_Notifier_Send(t *testing.T) {
	coreID := os.Getenv("WEIXIN_APP_CORP_ID")
	if coreID == "" {
		t.Skip("skip: WEIXIN_APP_CORP_ID not set")
	}

	aID := os.Getenv("WEIXIN_APP_AGENT_ID")
	agentID, err := strconv.Atoi(aID)
	if err != nil {
		t.Skipf("skip: WEIXIN_APP_AGENT_ID not set or invalid: %v", err)
	}

	agentSecret := os.Getenv("WEIXIN_APP_SECRET")
	if agentSecret == "" {
		t.Skip("skip: WEIXIN_APP_SECRET not set")
	}

	toUser := ""
	toParty := "2"
	toTag := ""
	tt := NewNotifer(coreID, agentID, agentSecret, toUser, toParty, toTag)

	msg := NewMsgMarkdown("# Hello World")
	if err := tt.Send(msg); err != nil {
		t.Fatalf("err: %v", err)
	}
}
