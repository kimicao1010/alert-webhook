package sqlitestore

import (
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/store"
)

// SendView 实现 store.SendStore（操作 sends 表）。
type SendView struct {
	s *Store
}

// Sends 返回发送记录存储视图（实现 store.SendStore）。
func (s *Store) Sends() *SendView { return &SendView{s: s} }

var _ store.SendStore = (*SendView)(nil)

// defaultSendLimit 发送记录保留上限（与 sendstore.JSONStore 一致）。
const defaultSendLimit = 1000

// Append 追加一条发送记录，并在事务内裁剪超限记录（保留最新 defaultSendLimit 条）。
func (v *SendView) Append(r store.SendRecord) error {
	v.s.mu.Lock()
	defer v.s.mu.Unlock()

	tx, err := v.s.db.Begin()
	if err != nil {
		return err
	}
	// 提交成功后回滚是 no-op；显式忽略其错误（ErrTxDone），满足 errcheck。
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`INSERT INTO sends(timestamp, channel, kind, status, error, alert_count, duration_ms, raw, title, text, markdown)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		r.Timestamp, r.Channel, r.Kind, r.Status, r.Error,
		r.AlertCount, r.Duration, r.Raw, r.Title, r.Text, r.Markdown,
	); err != nil {
		return err
	}
	// 裁剪：保留最新 limit 条（按 id 降序取前 limit）
	if _, err := tx.Exec(
		`DELETE FROM sends WHERE id NOT IN (
			SELECT id FROM sends ORDER BY id DESC LIMIT ?
		)`, defaultSendLimit,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// Query 按 offset/limit 分页查询，可按 channel/status 过滤；返回记录按 id 倒序（最新在前）。
func (v *SendView) Query(offset, limit int, channel, status string) ([]store.SendRecord, error) {
	where := ""
	args := []interface{}{}
	if channel != "" {
		where += " AND channel = ?"
		args = append(args, channel)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if len(where) > 0 {
		where = " WHERE " + where[5:]
	}
	if limit <= 0 {
		limit = 100000
	}
	if offset < 0 {
		offset = 0
	}
	query := `SELECT timestamp, channel, kind, status, error, alert_count, duration_ms, raw, title, text, markdown
		FROM sends` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := v.s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []store.SendRecord{}
	for rows.Next() {
		var r store.SendRecord
		if err := rows.Scan(&r.Timestamp, &r.Channel, &r.Kind, &r.Status, &r.Error,
			&r.AlertCount, &r.Duration, &r.Raw, &r.Title, &r.Text, &r.Markdown); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, rows.Err()
}
