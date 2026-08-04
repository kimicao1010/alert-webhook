package models

import (
	"strings"
	"testing"
)

const nodeDiskRaw = `{"receiver":"webhook","status":"firing","alerts":[{"status":"firing","labels":{"alertname":"NodeDiskUsage","device":"/dev/vda3","fstype":"ext4","instance":"kl-k3s-worker002","job":"nodes","mountpoint":"/","severity":"Warning"},"annotations":{"description":"实例磁盘: /dev/vda3使用率超过 60% (当前值为: 22%).挂载点: /","hostname":"kl-k3s-worker002","summary":"实例 kl-k3s-worker002 磁盘使用率过高"},"startsAt":"2026-08-04T04:43:31.991Z","endsAt":"0001-01-01T00:00:00Z","generatorURL":"http://prometheus-86f954989-pxwvd:9090/graph","fingerprint":"7aa70350721127d9"}],"groupLabels":{"alertname":"NodeDiskUsage","instance":"kl-k3s-worker002","job":"nodes"},"commonLabels":{"alertname":"NodeDiskUsage","device":"/dev/vda3","fstype":"ext4","instance":"kl-k3s-worker002","job":"nodes","mountpoint":"/","severity":"Warning"},"commonAnnotations":{"description":"实例磁盘: /dev/vda3使用率超过 60% (当前值为: 22%).挂载点: /","hostname":"kl-k3s-worker002","summary":"实例 kl-k3s-worker002 磁盘使用率过高"},"externalURL":"http://kl-prod-proxy01:9093","version":"4","groupKey":"{}:{alertname=\"NodeDiskUsage\", instance=\"kl-k3s-worker002\", job=\"nodes\"}","truncatedAlerts":0}`

func TestRenderCustomTmplRealPayload(t *testing.T) {
	fieldMap := map[string]string{
		"alertname": "alerts[0].labels.alertname",
		"severity":  "alerts[0].labels.severity",
		"instance":  "alerts[0].labels.instance",
		"summary":   "alerts[0].annotations.summary",
		"missing":   "alerts[0].labels.does_not_exist",
	}
	content := `{{ define "prom.title" }}{{ if eq (.Custom.severity | toLower) "warning" }}WARNING{{ end }} • {{ .Custom.alertname }}{{ end }}
{{ define "prom.text" }}{{ .Custom.instance }}: {{ .Custom.summary }}{{ end }}
{{ define "prom.markdown" }}**{{ .Custom.alertname }}** ({{ .Custom.instance }})
{{ if .Custom.missing }}HAS:{{ .Custom.missing }}{{ else }}MISSING-FIELD-OK{{ end }}
{{ .Custom.summary }}{{ end }}`

	payload, err := RenderCustomTmpl(content, fieldMap, []byte(nodeDiskRaw))
	if err != nil {
		t.Fatalf("RenderCustomTmpl: %v", err)
	}
	if !strings.Contains(payload.Title, "WARNING") {
		t.Errorf("title missing WARNING: %q", payload.Title)
	}
	if !strings.Contains(payload.Title, "NodeDiskUsage") {
		t.Errorf("title missing alertname: %q", payload.Title)
	}
	if !strings.Contains(payload.Markdown, "kl-k3s-worker002") {
		t.Errorf("markdown missing instance: %q", payload.Markdown)
	}
	if !strings.Contains(payload.Markdown, "MISSING-FIELD-OK") {
		t.Errorf("missing field should be zero, got: %q", payload.Markdown)
	}
	if !strings.Contains(payload.Text, "磁盘") || !strings.Contains(payload.Text, "使用率") {
		t.Errorf("text missing summary: %q", payload.Text)
	}
}
