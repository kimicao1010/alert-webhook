// Package channelstore 提供渠道配置的 JSON 文件存储，
// 实现 store.ChannelStore 接口。
package channelstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// JSONStore 以 data/channels/{channel}.json 形式存储渠道配置。
type JSONStore struct {
	dir string
	mu  sync.Mutex
}

// NewJSONStore 创建渠道配置存储，dir 为配置目录（如 <data-dir>/channels）。
func NewJSONStore(dir string) *JSONStore {
	return &JSONStore{dir: dir}
}

// safeChannel 校验渠道名合法性，防止路径穿越。
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

// List 返回全部已配置渠道名（按名称排序）。
func (s *JSONStore) List() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	res := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		res = append(res, strings.TrimSuffix(e.Name(), ".json"))
	}
	sort.Strings(res)
	return res, nil
}

// Get 返回指定渠道配置；不存在时返回空 map。
func (s *JSONStore) Get(channel string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.path(channel)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	cfg := map[string]string{}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse channel config %q failed: %w", channel, err)
	}
	return cfg, nil
}

// Save 保存渠道配置（创建或覆盖）。
func (s *JSONStore) Save(channel string, cfg map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.path(channel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, raw, 0600)
}

// Delete 删除渠道配置；不存在时不视为错误。
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
