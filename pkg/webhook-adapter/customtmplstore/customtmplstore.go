// Package customtmplstore 提供自定义模板的 JSON 文件存储，
// 实现 store.CustomTemplateStore 接口：每渠道一个 JSON 文件。
package customtmplstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/store"
)

// JSONStore 以 <dir>/{channel}.json 形式存储自定义模板。
type JSONStore struct {
	dir string
	mu  sync.Mutex
}

// NewJSONStore 创建自定义模板存储，dir 为配置目录（如 <data-dir>/custom-templates）。
func NewJSONStore(dir string) *JSONStore {
	return &JSONStore{dir: dir}
}

// safeChannel 校验渠道名合法性，防止路径穿越（与 channelstore 同规则）。
func safeChannel(channel string) error {
	if channel == "" {
		return fmt.Errorf("channel name must not be empty")
	}
	if strings.Contains(channel, "..") || strings.Contains(channel, "/") || strings.Contains(channel, "\\") {
		return fmt.Errorf("invalid channel name: %q", channel)
	}
	return nil
}

func (s *JSONStore) path(channel string) (string, error) {
	if err := safeChannel(channel); err != nil {
		return "", err
	}
	return filepath.Join(s.dir, channel+".json"), nil
}

// List 返回全部自定义模板（按渠道名排序）。
func (s *JSONStore) List() ([]store.CustomTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []store.CustomTemplate{}, nil
		}
		return nil, err
	}
	res := make([]store.CustomTemplate, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		ch := strings.TrimSuffix(e.Name(), ".json")
		t, err := s.loadLocked(ch)
		if err != nil {
			return nil, err
		}
		if t != nil {
			res = append(res, *t)
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Channel < res[j].Channel })
	return res, nil
}

// Get 返回指定渠道自定义模板；未配置时返回 nil。
func (s *JSONStore) Get(channel string) (*store.CustomTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked(channel)
}

func (s *JSONStore) loadLocked(channel string) (*store.CustomTemplate, error) {
	p, err := s.path(channel)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	t := &store.CustomTemplate{}
	if err := json.Unmarshal(raw, t); err != nil {
		return nil, fmt.Errorf("parse custom template %q failed: %w", channel, err)
	}
	if t.Channel == "" {
		t.Channel = channel
	}
	return t, nil
}

// Save 保存指定渠道自定义模板（创建或覆盖）。
func (s *JSONStore) Save(t store.CustomTemplate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.path(t.Channel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, raw, 0600)
}

// Delete 删除指定渠道自定义模板；不存在时不视为错误。
func (s *JSONStore) Delete(channel string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.path(channel)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}