package sqlitestore

import (
	"database/sql"
	"fmt"

	"github.com/kimicao1010/alert-webhook/pkg/models/templates"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/store"
)

// TemplateView 实现 store.TemplateStore（操作 templates 表）。
type TemplateView struct {
	s *Store
}

// Templates 返回模板存储视图（实现 store.TemplateStore）。
func (s *Store) Templates() *TemplateView { return &TemplateView{s: s} }

var _ store.TemplateStore = (*TemplateView)(nil)

// List 返回全部模板名（按名称排序）。
func (v *TemplateView) List() ([]string, error) {
	rows, err := v.s.db.Query(`SELECT name FROM templates ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		res = append(res, name)
	}
	return res, rows.Err()
}

// Get 返回指定模板内容；不存在时返回错误。
func (v *TemplateView) Get(name string) (string, error) {
	var content string
	err := v.s.db.QueryRow(`SELECT content FROM templates WHERE name = ?`, name).Scan(&content)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("template %q not found", name)
	}
	if err != nil {
		return "", err
	}
	return content, nil
}

// Save 保存模板内容（创建或覆盖）。
func (v *TemplateView) Save(name string, content string) error {
	_, err := v.s.db.Exec(
		`INSERT INTO templates(name, content) VALUES(?, ?)
		 ON CONFLICT(name) DO UPDATE SET content = excluded.content`,
		name, content)
	return err
}

// Delete 删除模板；不存在时不视为错误。
func (v *TemplateView) Delete(name string) error {
	_, err := v.s.db.Exec(`DELETE FROM templates WHERE name = ?`, name)
	return err
}

// EnsureInitialTemplates 首次启动把内置模板写入 templates 表；已存在则跳过（幂等，
// 不覆盖用户编辑），语义与 tmplstore.JSONStore.EnsureInitialTemplates 一致。
func (v *TemplateView) EnsureInitialTemplates() error {
	v.s.mu.Lock()
	defer v.s.mu.Unlock()

	builtin := map[string]string{}
	for ch, content := range templates.ChannelsDefaultTmplMapByLang["en"] {
		builtin[ch+".tmpl"] = content
	}
	for ch, content := range templates.ChannelsDefaultTmplMapByLang["zh"] {
		builtin[ch+".zh.tmpl"] = content
	}
	for name, content := range builtin {
		var exists int
		if err := v.s.db.QueryRow(`SELECT COUNT(1) FROM templates WHERE name = ?`, name).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			continue
		}
		if _, err := v.s.db.Exec(`INSERT INTO templates(name, content) VALUES(?, ?)`, name, content); err != nil {
			return fmt.Errorf("seed template %q failed: %w", name, err)
		}
	}
	return nil
}
