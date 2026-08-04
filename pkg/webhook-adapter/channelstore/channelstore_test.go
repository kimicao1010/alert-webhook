package channelstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndGet(t *testing.T) {
	dir := t.TempDir()
	s := NewJSONStore(filepath.Join(dir, "channels"))
	if err := s.Save("feishu", map[string]string{"token": "abc"}); err != nil {
		t.Fatal(err)
	}
	m, err := s.Get("feishu")
	if err != nil {
		t.Fatal(err)
	}
	if m["token"] != "abc" {
		t.Fatalf("unexpected: %v", m)
	}
}

func TestGetMissingReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	s := NewJSONStore(filepath.Join(dir, "channels"))
	m, err := s.Get("not-exist")
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Fatalf("expected empty map, got %v", m)
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	s := NewJSONStore(filepath.Join(dir, "channels"))
	for _, ch := range []string{"feishu", "dingtalk", "weixin"} {
		if err := s.Save(ch, map[string]string{"token": "x"}); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 channels, got %v", list)
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	s := NewJSONStore(filepath.Join(dir, "channels"))
	if err := s.Save("feishu", map[string]string{"token": "abc"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("feishu"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "channels", "feishu.json")); !os.IsNotExist(err) {
		t.Fatalf("file should be removed, err: %v", err)
	}
	// 删除不存在的渠道不报错
	if err := s.Delete("never-exists"); err != nil {
		t.Fatalf("delete missing should not error, got: %v", err)
	}
}

func TestPathTraversalRejected(t *testing.T) {
	dir := t.TempDir()
	s := NewJSONStore(filepath.Join(dir, "channels"))
	for _, bad := range []string{"../evil", "a/b", "..", ""} {
		if err := s.Save(bad, map[string]string{"token": "x"}); err == nil {
			t.Fatalf("Save(%q) should be rejected", bad)
		}
		if _, err := s.Get(bad); err == nil {
			t.Fatalf("Get(%q) should be rejected", bad)
		}
		if err := s.Delete(bad); err == nil {
			t.Fatalf("Delete(%q) should be rejected", bad)
		}
	}
}

func TestFilePermission(t *testing.T) {
	dir := t.TempDir()
	s := NewJSONStore(filepath.Join(dir, "channels"))
	if err := s.Save("feishu", map[string]string{"token": "abc"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "channels", "feishu.json"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("expected 0600, got %o", perm)
	}
}
