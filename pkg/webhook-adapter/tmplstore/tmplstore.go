// Package tmplstore 提供模板的 JSON 文件存储（plain text 文件），
// 实现 store.TemplateStore 接口，并在首次启动时从内置模板复制初始副本。
package tmplstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/kimicao1010/alert-webhook/pkg/models/templates"
)

// JSONStore 以 data/templates/{name} 形式存储模板文件。
type JSONStore struct {
	dir string
	mu  sync.Mutex
}

// NewJSONStore 创建模板存储，dir 为模板目录（如 <data-dir>/templates）。
func NewJSONStore(dir string) *JSONStore {
	return &JSONStore{dir: dir}
}

// safeName 校验模板名合法性，防止路径穿越。
func safeName(name string) error {
	if name == "" {
		return fmt.Errorf("template name must not be empty")
	}
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("invalid template name: %q", name)
	}
	return nil
}

func (s *JSONStore) path(name string) (string, error) {
	if err := safeName(name); err != nil {
		return "", err
	}
	return filepath.Join(s.dir, name), nil
}

// EnsureInitialTemplates 从内置模板复制初始副本到模板目录；
// 已存在的文件跳过（不覆盖用户编辑），幂等。
// 同时清理旧版残留：删除 <channel>.zh.tmpl（去语言化后不再使用）。
func (s *JSONStore) EnsureInitialTemplates() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}

	// 清理旧版 zh 残留模板（去语言化后不再加载，避免误导）
	if entries, err := os.ReadDir(s.dir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.HasSuffix(e.Name(), ".zh.tmpl") {
				_ = os.Remove(filepath.Join(s.dir, e.Name()))
			}
		}
	}

	// 内置默认模板：所有渠道共用一套（内容来自 default.tmpl）
	builtin := map[string]string{}
	defaultContent := templates.DefaultTmpl
	for _, ch := range []string{"dingtalk", "feishu", "weixin", "weixinapp"} {
		builtin[ch+".tmpl"] = defaultContent
	}

	for name, content := range builtin {
		p, err := s.path(name)
		if err != nil {
			return err
		}
		if err := writeIfAbsent(p, content); err != nil {
			return err
		}
	}
	return nil
}

// writeIfAbsent 仅在文件不存在时写入（0600 权限）。
func writeIfAbsent(path string, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, []byte(content), 0600)
}

// List 返回全部模板名（按名称排序）。
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
		if e.IsDir() {
			continue
		}
		res = append(res, e.Name())
	}
	sort.Strings(res)
	return res, nil
}

// Get 返回指定模板内容；不存在时返回错误。
func (s *JSONStore) Get(name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.path(name)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// Save 保存模板内容（创建或覆盖）。
func (s *JSONStore) Save(name string, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.path(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content), 0600)
}

// Delete 删除模板；不存在时不视为错误。
func (s *JSONStore) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
