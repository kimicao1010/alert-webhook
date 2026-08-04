package sendstore

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/store"
)

func TestAppendAndQuery(t *testing.T) {
	dir := t.TempDir()
	s := NewJSONStore(filepath.Join(dir, "sends.json"), 100)
	if err := s.Append(store.SendRecord{Timestamp: time.Now().Unix(), Channel: "feishu", Status: "success"}); err != nil {
		t.Fatal(err)
	}
	recs, err := s.Query(0, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1, got %d", len(recs))
	}
}

func TestQueryFilterByChannelAndStatus(t *testing.T) {
	dir := t.TempDir()
	s := NewJSONStore(filepath.Join(dir, "sends.json"), 100)
	base := time.Now().Unix()
	if err := s.Append(store.SendRecord{Timestamp: base, Channel: "feishu", Status: "success"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Append(store.SendRecord{Timestamp: base + 1, Channel: "dingtalk", Status: "failure", Error: "boom"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Append(store.SendRecord{Timestamp: base + 2, Channel: "feishu", Status: "failure", Error: "nope"}); err != nil {
		t.Fatal(err)
	}

	// 按渠道过滤
	feishuRecs, err := s.Query(0, 10, "feishu", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(feishuRecs) != 2 {
		t.Fatalf("expected 2 feishu records, got %d", len(feishuRecs))
	}

	// 按状态过滤
	failedRecs, err := s.Query(0, 10, "", "failure")
	if err != nil {
		t.Fatal(err)
	}
	if len(failedRecs) != 2 {
		t.Fatalf("expected 2 failure records, got %d", len(failedRecs))
	}

	// 组合过滤
	feishuFailed, err := s.Query(0, 10, "feishu", "failure")
	if err != nil {
		t.Fatal(err)
	}
	if len(feishuFailed) != 1 || feishuFailed[0].Error != "nope" {
		t.Fatalf("unexpected: %+v", feishuFailed)
	}
}

func TestQueryNewestFirst(t *testing.T) {
	dir := t.TempDir()
	s := NewJSONStore(filepath.Join(dir, "sends.json"), 100)
	base := time.Now().Unix()
	for i := int64(0); i < 5; i++ {
		if err := s.Append(store.SendRecord{Timestamp: base + i, Channel: "feishu", Status: "success"}); err != nil {
			t.Fatal(err)
		}
	}
	recs, err := s.Query(0, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 5 {
		t.Fatalf("expected 5, got %d", len(recs))
	}
	// 最新的在前
	if recs[0].Timestamp != base+4 || recs[4].Timestamp != base {
		t.Fatalf("expected newest-first order, got first=%d last=%d", recs[0].Timestamp, recs[4].Timestamp)
	}
}

func TestTrimToLimit(t *testing.T) {
	dir := t.TempDir()
	s := NewJSONStore(filepath.Join(dir, "sends.json"), 3)
	for i := 0; i < 10; i++ {
		if err := s.Append(store.SendRecord{Timestamp: time.Now().Unix() + int64(i), Channel: "feishu", Status: "success"}); err != nil {
			t.Fatal(err)
		}
	}
	recs, err := s.Query(0, 100, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 3 {
		t.Fatalf("expected trimmed to 3, got %d", len(recs))
	}
	// 保留最新 3 条（timestamp 7,8,9）
	if recs[0].Timestamp != time.Now().Unix()+9 {
		t.Fatalf("expected newest kept, got %d", recs[0].Timestamp)
	}
}

func TestPagination(t *testing.T) {
	dir := t.TempDir()
	s := NewJSONStore(filepath.Join(dir, "sends.json"), 100)
	base := time.Now().Unix()
	for i := int64(0); i < 10; i++ {
		if err := s.Append(store.SendRecord{Timestamp: base + i, Channel: "feishu", Status: "success"}); err != nil {
			t.Fatal(err)
		}
	}
	page1, err := s.Query(0, 4, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page1) != 4 {
		t.Fatalf("page1 expected 4, got %d", len(page1))
	}
	page2, err := s.Query(4, 4, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 4 {
		t.Fatalf("page2 expected 4, got %d", len(page2))
	}
	if page1[0].Timestamp == page2[0].Timestamp {
		t.Fatal("pages should not overlap")
	}
}

func TestPersistenceAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sends.json")
	s1 := NewJSONStore(p, 100)
	if err := s1.Append(store.SendRecord{Timestamp: time.Now().Unix(), Channel: "feishu", Status: "success"}); err != nil {
		t.Fatal(err)
	}
	// 新实例读同一文件
	s2 := NewJSONStore(p, 100)
	recs, err := s2.Query(0, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 persisted record, got %d", len(recs))
	}
}
