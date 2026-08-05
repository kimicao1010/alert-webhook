package api

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kimicao1010/alert-webhook/pkg/senders"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/models"
)

// ---- mock 存储 ----

type mockChannelStore struct {
	mu   sync.Mutex
	cfgs map[string]map[string]string
}

func newMockChannelStore() *mockChannelStore {
	return &mockChannelStore{cfgs: map[string]map[string]string{}}
}

func (m *mockChannelStore) List() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []string{}
	for ch := range m.cfgs {
		out = append(out, ch)
	}
	return out, nil
}

func (m *mockChannelStore) Get(ch string) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg, ok := m.cfgs[ch]; ok {
		cp := map[string]string{}
		for k, v := range cfg {
			cp[k] = v
		}
		return cp, nil
	}
	return map[string]string{}, nil
}

func (m *mockChannelStore) Save(ch string, cfg map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfgs[ch] = cfg
	return nil
}

func (m *mockChannelStore) Delete(ch string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cfgs, ch)
	return nil
}

// ---- mock sender ----

// mockSender 前 failures 次 Send 失败，之后成功；记录调用次数与收到的 payload。
type mockSender struct {
	failures int
	calls    int
	payload  *models.Payload
}

func (m *mockSender) Send(p *models.Payload) error {
	m.calls++
	m.payload = p
	if m.calls <= m.failures {
		return errors.New("mock send failure")
	}
	return nil
}

func (m *mockSender) SendMsg(msg interface{}) error                  { return nil }
func (m *mockSender) SendMsgT(msgType string, msg interface{}) error { return nil }

// failSender 永远失败。
type failSender struct{ calls int }

func (m *failSender) Send(p *models.Payload) error                   { m.calls++; return errors.New("always fail") }
func (m *failSender) SendMsg(msg interface{}) error                  { return nil }
func (m *failSender) SendMsgT(msgType string, msg interface{}) error { return nil }

// ---- 测试装配 ----

// registerMockChannels 注册测试渠道的 mock sender creator，并返回各 sender 句柄。
func registerMockChannels(t *testing.T, channels []string) map[string]*mockSender {
	t.Helper()
	created := map[string]*mockSender{}
	for _, ch := range channels {
		s := &mockSender{}
		created[ch] = s
		senders.RegisterChannelsSenderCreator(ch, func(cfg map[string]string) (models.Sender, error) {
			return s, nil
		})
	}
	return created
}

func newTestController(t *testing.T, channels []string) (*Controller, map[string]*mockSender) {
	t.Helper()
	cs := newMockChannelStore()
	for _, ch := range channels {
		_ = cs.Save(ch, map[string]string{"token": "x"})
	}
	created := registerMockChannels(t, channels)
	c := NewController("test")
	c.WithChannelStore(cs)
	c.WithFailoverDisabled(false)
	// 测试用毫秒级 backoff，避免真实 1s/2s/3s 等待
	c.retryBackoffs = []time.Duration{time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond}
	return c, created
}

// ---- 测试用例 ----

func TestFailoverChannelsExcludesPrimary(t *testing.T) {
	c, _ := newTestController(t, []string{"feishu", "weixin", "dingtalk"})
	got := c.failoverChannels("feishu")
	if len(got) != 2 {
		t.Fatalf("expected 2 failover channels, got %v", got)
	}
	for _, ch := range got {
		if ch == "feishu" {
			t.Fatal("primary channel should be excluded")
		}
	}
}

func TestSendWithFailover_BackupSuccess(t *testing.T) {
	c, created := newTestController(t, []string{"primary", "backup"})
	// 主渠道永远失败，备用渠道首次失败后成功
	primary := &failSender{}
	senders.RegisterChannelsSenderCreator("primary", func(cfg map[string]string) (models.Sender, error) {
		return primary, nil
	})
	created["backup"].failures = 0

	payload := &models.Payload{Title: "t", Markdown: "m"}
	err, used := c.sendWithFailover("primary", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if used != "backup" {
		t.Fatalf("expected backup channel, got %q", used)
	}
	if created["backup"].calls != 1 {
		t.Fatalf("backup should be called once, got %d", created["backup"].calls)
	}
	if created["backup"].payload != payload {
		t.Fatal("backup should receive same payload")
	}
}

func TestSendWithFailover_AllFail(t *testing.T) {
	c, _ := newTestController(t, []string{"primary", "backup1", "backup2"})
	// 所有渠道都注册为 failSender
	failers := map[string]*failSender{}
	for _, ch := range []string{"primary", "backup1", "backup2"} {
		s := &failSender{}
		failers[ch] = s
		senders.RegisterChannelsSenderCreator(ch, func(cfg map[string]string) (models.Sender, error) {
			return s, nil
		})
	}

	payload := &models.Payload{Title: "t"}
	err, used := c.sendWithFailover("primary", payload)
	if err == nil {
		t.Fatal("expected error when all channels fail")
	}
	if used != "" {
		t.Fatalf("expected empty used channel, got %q", used)
	}
	if failers["backup1"].calls < 1 || failers["backup2"].calls < 1 {
		t.Fatalf("all backups should be tried: backup1=%d backup2=%d", failers["backup1"].calls, failers["backup2"].calls)
	}
}

func TestSendWithFailover_NoBackup(t *testing.T) {
	c, _ := newTestController(t, []string{"only"})
	payload := &models.Payload{Title: "t"}
	err, used := c.sendWithFailover("only", payload)
	if err == nil {
		t.Fatal("expected error when no backup channel exists")
	}
	if used != "" {
		t.Fatalf("expected empty used channel, got %q", used)
	}
}
