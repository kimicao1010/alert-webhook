package sqlitestore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/store"
)

// ChannelView 实现 store.ChannelStore（操作 channels 表）。
type ChannelView struct {
	s *Store
}

// Channels 返回渠道配置存储视图（实现 store.ChannelStore）。
func (s *Store) Channels() *ChannelView { return &ChannelView{s: s} }

var _ store.ChannelStore = (*ChannelView)(nil)

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

// List 返回全部已配置渠道名（按名称排序）。
func (v *ChannelView) List() ([]string, error) {
	rows, err := v.s.db.Query(`SELECT channel FROM channels ORDER BY channel`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []string{}
	for rows.Next() {
		var ch string
		if err := rows.Scan(&ch); err != nil {
			return nil, err
		}
		res = append(res, ch)
	}
	return res, rows.Err()
}

// Get 返回指定渠道的配置；渠道不存在时返回空 map（非错误）。
func (v *ChannelView) Get(channel string) (map[string]string, error) {
	if err := safeChannel(channel); err != nil {
		return nil, err
	}
	var raw string
	err := v.s.db.QueryRow(`SELECT config FROM channels WHERE channel = ?`, channel).Scan(&raw)
	if err == sql.ErrNoRows {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	cfg := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("parse channel config %q failed: %w", channel, err)
	}
	return cfg, nil
}

// Save 保存渠道配置（创建或覆盖）。
func (v *ChannelView) Save(channel string, cfg map[string]string) error {
	if err := safeChannel(channel); err != nil {
		return err
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = v.s.db.Exec(
		`INSERT INTO channels(channel, config) VALUES(?, ?)
		 ON CONFLICT(channel) DO UPDATE SET config = excluded.config`,
		channel, string(raw))
	return err
}

// Delete 删除渠道配置；渠道不存在时不视为错误。
func (v *ChannelView) Delete(channel string) error {
	if err := safeChannel(channel); err != nil {
		return err
	}
	_, err := v.s.db.Exec(`DELETE FROM channels WHERE channel = ?`, channel)
	return err
}
