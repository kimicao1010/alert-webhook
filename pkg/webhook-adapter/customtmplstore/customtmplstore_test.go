package customtmplstore

import (
	"path/filepath"
	"testing"

	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/store"
)

func newStore(t *testing.T) *JSONStore {
	t.Helper()
	return NewJSONStore(filepath.Join(t.TempDir(), "custom-templates"))
}

func TestCRUD(t *testing.T) {
	s := newStore(t)
	if got, _ := s.List(); len(got) != 0 {
		t.Fatalf("initial list len = %d, want 0", len(got))
	}
	// 未配置返回 nil 而非错误
	tmpl, err := s.Get("weixinapp")
	if err != nil || tmpl != nil {
		t.Fatalf("Get missing = %v, %v; want nil, nil", tmpl, err)
	}
	// 保存
	ct := store.CustomTemplate{
		Channel: "weixinapp",
		Content: `{{ define "prom.title" }}test{{ end }}`,
		FieldMap: map[string]string{
			"severity": "alerts[0].labels.severity",
		},
	}
	if err := s.Save(ct); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// 读回
	got, err := s.Get("weixinapp")
	if err != nil || got == nil {
		t.Fatalf("Get after save = %v, %v", got, err)
	}
	if got.FieldMap["severity"] != "alerts[0].labels.severity" {
		t.Fatalf("FieldMap mismatch: %v", got.FieldMap)
	}
	// 覆盖
	ct.Content = `{{ define "prom.title" }}updated{{ end }}`
	if err := s.Save(ct); err != nil {
		t.Fatalf("Save update: %v", err)
	}
	got, _ = s.Get("weixinapp")
	if got.Content != ct.Content {
		t.Fatalf("content not updated: %q", got.Content)
	}
	// 列表
	list, _ := s.List()
	if len(list) != 1 || list[0].Channel != "weixinapp" {
		t.Fatalf("List = %v", list)
	}
	// 删除
	if err := s.Delete("weixinapp"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if tmpl, _ := s.Get("weixinapp"); tmpl != nil {
		t.Fatal("Get after delete should be nil")
	}
	// 删除不存在的渠道不报错
	if err := s.Delete("nonexistent"); err != nil {
		t.Fatalf("Delete missing: %v", err)
	}
}

func TestPathTraversal(t *testing.T) {
	s := newStore(t)
	bad := store.CustomTemplate{Channel: "../evil", Content: "x"}
	if err := s.Save(bad); err == nil {
		t.Fatal("Save with ../ channel should fail")
	}
	if _, err := s.Get("../evil"); err == nil {
		t.Fatal("Get with ../ channel should fail")
	}
}
