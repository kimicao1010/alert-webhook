package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/models"
)

// RenderCustomTmpl 用自定义模板渲染三段 payload（prom.title/text/markdown）。
// rawBody 为调用源原始 JSON；FieldMap 声明「模板变量名 -> JSON 路径」，
// 提取结果以 .Custom.<var> 暴露给模板（缺失路径为零值 nil，模板可用 if/eq 判断）。
func RenderCustomTmpl(content string, fieldMap map[string]string, rawBody []byte) (*models.Payload, error) {
	body := map[string]any{}
	if len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, &body); err != nil {
			return nil, fmt.Errorf("parse raw body failed: %w", err)
		}
	}

	custom := map[string]any{}
	for varName, path := range fieldMap {
		custom[varName] = extractByPath(body, path)
	}
	data := map[string]any{"Custom": custom}

	tpl, err := template.New(topLevelTemplateName).
		Funcs(defaultFuncs).
		Option("missingkey=zero").
		Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse custom template failed: %w", err)
	}

	payload := &models.Payload{}
	for _, section := range []struct {
		name string
		dst  *string
	}{
		{"prom.title", &payload.Title},
		{"prom.text", &payload.Text},
		{"prom.markdown", &payload.Markdown},
	} {
		var buf bytes.Buffer
		if err := tpl.ExecuteTemplate(&buf, section.name, data); err != nil {
			return nil, fmt.Errorf("execute %s failed: %w", section.name, err)
		}
		*section.dst = buf.String()
	}
	return payload, nil
}
