package models

import "testing"

func TestRenderTemplateContent(t *testing.T) {
	m := &AlertmanagerWebhookMessage{Status: "firing"}
	out, err := RenderTemplateContent(`{{ define "prom.title" }}[{{ .Status }}]{{ end }}`, "prom.title", m)
	if err != nil {
		t.Fatal(err)
	}
	if out != "[firing]" {
		t.Fatalf("unexpected: %s", out)
	}
}

func TestRenderTemplateContentWithFuncs(t *testing.T) {
	m := &AlertmanagerWebhookMessage{Status: "firing"}
	// 验证 defaultFuncs（toUpper）在预览渲染中可用
	out, err := RenderTemplateContent(`{{ define "prom.title" }}[{{ .Status | toUpper }}]{{ end }}`, "prom.title", m)
	if err != nil {
		t.Fatal(err)
	}
	if out != "[FIRING]" {
		t.Fatalf("unexpected: %s", out)
	}
}

func TestRenderTemplateContentMissingkeyZero(t *testing.T) {
	// missingkey=zero：map 缺失 key 渲染为空而非报错
	data := map[string]string{"status": "firing"}
	out, err := RenderTemplateContent(`{{ define "prom.title" }}[{{ .status }}]{{ .notExist }}{{ end }}`, "prom.title", data)
	if err != nil {
		t.Fatal(err)
	}
	if out != "[firing]" {
		t.Fatalf("unexpected: %s", out)
	}
}
