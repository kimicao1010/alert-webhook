package models

import (
	"encoding/json"
	"testing"
)

// 用用户提供的真实 Alertmanager 负载验证 weixinapp 渲染。
// 期望：Alert Level 显示 WARNING（severity "Warning" 大小写归一化）；
// Alert Instance 显示 kl-k3s-worker002（__alertinstance 不再被缺失键短路）。
func TestRealPayloadWeixinapp(t *testing.T) {
	raw := `{"receiver":"webhook","status":"firing","alerts":[{"status":"firing","labels":{"alertname":"NodeDiskUsage","device":"/dev/vda3","fstype":"ext4","instance":"kl-k3s-worker002","job":"nodes","mountpoint":"/","severity":"Warning"},"annotations":{"description":"实例磁盘: /dev/vda3使用率超过 60% (当前值为: 22%).挂载点: /","hostname":"kl-k3s-worker002","summary":"实例 kl-k3s-worker002 磁盘使用率过高"},"startsAt":"2026-08-04T04:43:31.991Z","endsAt":"0001-01-01T00:00:00Z","generatorURL":"http://prometheus-86f954989-pxwvd:9090/graph","fingerprint":"7aa70350721127d9"}],"groupLabels":{"alertname":"NodeDiskUsage","instance":"kl-k3s-worker002","job":"nodes"},"commonLabels":{"alertname":"NodeDiskUsage","device":"/dev/vda3","fstype":"ext4","instance":"kl-k3s-worker002","job":"nodes","mountpoint":"/","severity":"Warning"},"commonAnnotations":{"description":"实例磁盘: /dev/vda3使用率超过 60% (当前值为: 22%).挂载点: /","hostname":"kl-k3s-worker002","summary":"实例 kl-k3s-worker002 磁盘使用率过高"},"externalURL":"http://kl-prod-proxy01:9093","version":"4","groupKey":"{}:{alertname=\"NodeDiskUsage\", instance=\"kl-k3s-worker002\", job=\"nodes\"}","truncatedAlerts":0}`

	m := &AlertmanagerWebhookMessage{}
	if err := json.Unmarshal([]byte(raw), m); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}

	// 渲染 weixinapp 渠道的 title 和 markdown
	title, err := m.RenderTmpl("weixinapp", "prom.title")
	if err != nil {
		t.Fatalf("RenderTmpl title: %v", err)
	}
	markdown, err := m.RenderTmpl("weixinapp", "prom.markdown")
	if err != nil {
		t.Fatalf("RenderTmpl markdown: %v", err)
	}

	t.Logf("TITLE: %s", title)
	t.Logf("MARKDOWN:\n%s", markdown)

	// 断言：title 包含 WARNING 级别
	if !contains(title, "WARNING") {
		t.Errorf("title 缺少 WARNING 级别: %s", title)
	}
	// 断言：markdown 包含实例与 WARNING 级别
	if !contains(markdown, "kl-k3s-worker002") {
		t.Errorf("markdown 缺少 instance: %s", markdown)
	}
	if !contains(markdown, "WARNING") {
		t.Errorf("markdown 缺少 WARNING 级别: %s", markdown)
	}
	// 断言：不再输出空字段标记
	if contains(markdown, "Alert Instance </font>: ``") {
		t.Errorf("Alert Instance 仍为空: %s", markdown)
	}
	if contains(markdown, "Alert Level </font>: ") && contains(markdown, "Alert Level </font>: \n") {
		// 空 level 的判定：Level 后直接换行（无值）
		t.Errorf("Alert Level 仍为空: %s", markdown)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
