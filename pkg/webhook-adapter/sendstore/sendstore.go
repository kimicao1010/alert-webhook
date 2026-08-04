// Package sendstore 提供发送记录的 JSON 文件存储，
// 实现 store.SendStore 接口：原子写、超限裁剪、按渠道/状态筛选、倒序分页。
package sendstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/store"
)

// JSONStore 以单个 JSON 文件存储全部发送记录。
type JSONStore struct {
	path  string
	limit int
	mu    sync.Mutex
}

// NewJSONStore 创建发送记录存储；limit 为保留的最大记录条数（超出裁剪最旧）。
func NewJSONStore(path string, limit int) *JSONStore {
	if limit <= 0 {
		limit = 1000
	}
	return &JSONStore{path: path, limit: limit}
}

// load 读取全部记录（文件不存在时返回空 slice）。
func (s *JSONStore) load() ([]store.SendRecord, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []store.SendRecord{}, nil
		}
		return nil, err
	}
	var recs []store.SendRecord
	if err := json.Unmarshal(raw, &recs); err != nil {
		return nil, fmt.Errorf("parse send records failed: %w", err)
	}
	if recs == nil {
		recs = []store.SendRecord{}
	}
	return recs, nil
}

// save 原子写：先写临时文件再 rename，避免写一半损坏数据。
func (s *JSONStore) save(recs []store.SendRecord) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Append 追加一条记录，裁剪超限的最旧记录后原子写盘。
func (s *JSONStore) Append(r store.SendRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	recs, err := s.load()
	if err != nil {
		return err
	}
	recs = append(recs, r)
	// 裁剪：保留最新 limit 条
	if len(recs) > s.limit {
		recs = recs[len(recs)-s.limit:]
	}
	return s.save(recs)
}

// Query 倒序（最新在前）分页查询，可按 channel/status 过滤。
func (s *JSONStore) Query(offset, limit int, channel, status string) ([]store.SendRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	recs, err := s.load()
	if err != nil {
		return nil, err
	}

	// 倒序：最新的在前
	filtered := make([]store.SendRecord, 0, len(recs))
	for i := len(recs) - 1; i >= 0; i-- {
		r := recs[i]
		if channel != "" && r.Channel != channel {
			continue
		}
		if status != "" && r.Status != status {
			continue
		}
		filtered = append(filtered, r)
	}

	// 分页
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = len(filtered)
	}
	if offset >= len(filtered) {
		return []store.SendRecord{}, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[offset:end], nil
}
