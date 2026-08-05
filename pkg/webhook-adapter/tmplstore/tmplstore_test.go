package tmplstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureInitialTemplates(t *testing.T) {
	dir := t.TempDir()
	s := NewJSONStore(filepath.Join(dir, "templates"))
	if err := s.EnsureInitialTemplates(); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"feishu.tmpl", "dingtalk.tmpl", "weixin.tmpl", "weixinapp.tmpl"} {
		if _, err := os.Stat(filepath.Join(dir, "templates", f)); os.IsNotExist(err) {
			t.Fatalf("%s not created", f)
		}
	}
	// 再次调用应幂等，不报错、不覆盖
	if err := s.EnsureInitialTemplates(); err != nil {
		t.Fatal(err)
	}
}

func TestSaveAndGet(t *testing.T) {
	dir := t.TempDir()
	s := NewJSONStore(filepath.Join(dir, "templates"))
	if err := s.Save("feishu.tmpl", "{{ define \"prom.title\" }}hello{{ end }}"); err != nil {
		t.Fatal(err)
	}
	content, err := s.Get("feishu.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if content != "{{ define \"prom.title\" }}hello{{ end }}" {
		t.Fatalf("unexpected: %s", content)
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	s := NewJSONStore(filepath.Join(dir, "templates"))
	if err := s.EnsureInitialTemplates(); err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("expected non-empty template list")
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	s := NewJSONStore(filepath.Join(dir, "templates"))
	if err := s.Save("custom.tmpl", "x"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("custom.tmpl"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("custom.tmpl"); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestPathTraversalRejected(t *testing.T) {
	dir := t.TempDir()
	s := NewJSONStore(filepath.Join(dir, "templates"))
	for _, bad := range []string{"../evil.tmpl", "a/b.tmpl", "..", ""} {
		if err := s.Save(bad, "x"); err == nil {
			t.Fatalf("Save(%q) should be rejected", bad)
		}
		if _, err := s.Get(bad); err == nil {
			t.Fatalf("Get(%q) should be rejected", bad)
		}
	}
}
