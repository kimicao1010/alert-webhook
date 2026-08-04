package models

import (
	"encoding/json"
	"testing"
)

func mustBody(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func TestExtractByPath(t *testing.T) {
	body := mustBody(t, `{
		"receiver": "webhook",
		"status": "firing",
		"alerts": [
			{"labels": {"severity": "Warning", "instance": "kl-k3s-worker002"}, "annotations": {"summary": "disk high"}},
			{"labels": {"severity": "critical"}}
		],
		"groupLabels": {"alertname": "NodeDiskUsage"}
	}`)

	cases := []struct {
		path string
		want any
	}{
		{"receiver", "webhook"},
		{"status", "firing"},
		{"alerts[0].labels.severity", "Warning"},
		{"alerts[0].labels.instance", "kl-k3s-worker002"},
		{"alerts[1].labels.severity", "critical"},
		{"alerts[0].annotations.summary", "disk high"},
		{"groupLabels.alertname", "NodeDiskUsage"},
		// 缺失路径 -> nil
		{"alerts[5].labels.severity", nil},
		{"alerts[0].labels.nonexistent", nil},
		{"nonexistent", nil},
		{"alerts[0].labels.severity.deep", nil},
		// 类型不匹配 -> nil
		{"receiver.deep", nil},
	}
	for _, c := range cases {
		got := extractByPath(body, c.path)
		if got != c.want {
			t.Errorf("extractByPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestParsePathEdgeCases(t *testing.T) {
	// "alerts[0].labels.severity" = alerts / [0] / labels / severity 共 4 段
	//（计划文档误写为 3，此处按实际语义修正）
	if segs := parsePath("alerts[0].labels.severity"); len(segs) != 4 {
		t.Fatalf("parse len = %d, want 4 (%v)", len(segs), segs)
	}
	// "a[0][1]" = key a + [0] + [1] 共 3 段
	//（计划文档误写为 2 段纯索引，此处按实际语义修正）
	if segs := parsePath("a[0][1]"); len(segs) != 3 || segs[0].key != "a" || !segs[1].isIndex || segs[1].index != 0 || !segs[2].isIndex || segs[2].index != 1 {
		t.Fatalf("nested index parse = %v", segs)
	}
	if segs := parsePath(""); len(segs) != 0 {
		t.Fatalf("empty path = %v", segs)
	}
}