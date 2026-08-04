package sqlitestore

import (
	"path/filepath"
	"testing"

	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/store"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// 重复打开同一库文件不应报错（幂等建表）
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	s2.Close()
}

func TestChannelStoreCRUD(t *testing.T) {
	s := newTestStore(t)
	ch := s.Channels()
	if got, _ := ch.List(); len(got) != 0 {
		t.Fatalf("initial list = %v, want empty", got)
	}
	if cfg, _ := ch.Get("weixinapp"); len(cfg) != 0 {
		t.Fatalf("Get missing channel = %v, want empty map", cfg)
	}
	if err := ch.Save("weixinapp", map[string]string{"corp_id": "x"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	cfg, err := ch.Get("weixinapp")
	if err != nil || cfg["corp_id"] != "x" {
		t.Fatalf("Get after save = %v, %v", cfg, err)
	}
	// 覆盖保存
	if err := ch.Save("weixinapp", map[string]string{"corp_id": "y", "agent_id": "100001"}); err != nil {
		t.Fatalf("Save update: %v", err)
	}
	cfg, _ = ch.Get("weixinapp")
	if cfg["corp_id"] != "y" || cfg["agent_id"] != "100001" {
		t.Fatalf("Get after update = %v", cfg)
	}
	// 列表排序
	if err := ch.Save("dingtalk", map[string]string{"token": "***"}); err != nil {
		t.Fatalf("Save dingtalk: %v", err)
	}
	list, _ := ch.List()
	if len(list) != 2 || list[0] != "dingtalk" || list[1] != "weixinapp" {
		t.Fatalf("List = %v, want [dingtalk weixinapp]", list)
	}
	// 删除
	if err := ch.Delete("weixinapp"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if cfg, _ := ch.Get("weixinapp"); len(cfg) != 0 {
		t.Fatalf("Get after delete = %v, want empty", cfg)
	}
	// 删除不存在的渠道不报错
	if err := ch.Delete("nonexistent"); err != nil {
		t.Fatalf("Delete missing: %v", err)
	}
	// 路径穿越防护
	if err := ch.Save("../evil", map[string]string{}); err == nil {
		t.Fatal("Save with ../ channel should fail")
	}
	if _, err := ch.Get("../evil"); err == nil {
		t.Fatal("Get with ../ channel should fail")
	}
}

func TestTemplateStoreCRUDAndEnsure(t *testing.T) {
	s := newTestStore(t)
	tmpl := s.Templates()
	if err := tmpl.EnsureInitialTemplates(); err != nil {
		t.Fatalf("EnsureInitialTemplates: %v", err)
	}
	list, _ := tmpl.List()
	if len(list) == 0 {
		t.Fatal("EnsureInitialTemplates produced no templates")
	}
	// 幂等：再次执行不报错且不覆盖用户修改
	if err := tmpl.Save(list[0], "custom content"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := tmpl.EnsureInitialTemplates(); err != nil {
		t.Fatalf("second EnsureInitialTemplates: %v", err)
	}
	got, _ := tmpl.Get(list[0])
	if got != "custom content" {
		t.Fatalf("EnsureInitialTemplates overwrote user edit: %q", got)
	}
	// Get 不存在的模板返回错误
	if _, err := tmpl.Get("no-such.tmpl"); err == nil {
		t.Fatal("Get missing template should error")
	}
	// 删除
	if err := tmpl.Delete(list[0]); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := tmpl.Get(list[0]); err == nil {
		t.Fatal("Get after delete should error")
	}
}

func TestSendStoreAppendTrimAndQuery(t *testing.T) {
	s := newTestStore(t)
	sends := s.Sends()
	// 写入超出 limit 的记录，验证裁剪
	for i := 0; i < defaultSendLimit+10; i++ {
		if err := sends.Append(store.SendRecord{
			Timestamp:  int64(i),
			Channel:    "weixinapp",
			Kind:       "real",
			Status:     "success",
			AlertCount: 1,
			Raw:        `{"alerts":[]}`,
			Title:      "t",
			Text:       "x",
			Markdown:   "m",
		}); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	recs, err := sends.Query(0, 100000, "", "")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(recs) != defaultSendLimit {
		t.Fatalf("after trim len = %d, want %d", len(recs), defaultSendLimit)
	}
	// 倒序：最新在前（timestamp 最大）
	if recs[0].Timestamp != int64(defaultSendLimit+9) {
		t.Fatalf("newest timestamp = %d, want %d", recs[0].Timestamp, defaultSendLimit+9)
	}
	// 最旧保留的是裁剪边界（前 10 条被裁掉）
	if recs[len(recs)-1].Timestamp != 10 {
		t.Fatalf("oldest kept timestamp = %d, want 10", recs[len(recs)-1].Timestamp)
	}
	// 内容快照字段完整保留
	if recs[0].Raw != `{"alerts":[]}` || recs[0].Title != "t" || recs[0].Text != "x" || recs[0].Markdown != "m" {
		t.Fatalf("content snapshot lost: %+v", recs[0])
	}
	// 分页 + 过滤
	page, _ := sends.Query(0, 10, "weixinapp", "success")
	if len(page) != 10 {
		t.Fatalf("page size = %d, want 10", len(page))
	}
	page2, _ := sends.Query(10, 10, "weixinapp", "success")
	if len(page2) != 10 || page2[0].Timestamp == page[0].Timestamp {
		t.Fatalf("offset page overlap: first=%d prev first=%d", page2[0].Timestamp, page[0].Timestamp)
	}
	empty, _ := sends.Query(0, 10, "weixinapp", "failure")
	if len(empty) != 0 {
		t.Fatalf("status filter = %d, want 0", len(empty))
	}
	emptyCh, _ := sends.Query(0, 10, "dingtalk", "")
	if len(emptyCh) != 0 {
		t.Fatalf("channel filter = %d, want 0", len(emptyCh))
	}
}
