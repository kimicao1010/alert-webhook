// Package sqlitestore 提供 SQLite 单文件存储，同时实现 store.ChannelStore /
// store.TemplateStore / store.SendStore 三个接口，作为默认存储后端。
// 驱动为 modernc.org/sqlite（纯 Go、无 CGO），保持交叉编译能力。
//
// 结构：Store 为核心（持有 db 与锁），对外通过 Channels()/Templates()/Sends()
// 三个视图分别实现三接口，避免 ChannelStore.List 与 TemplateStore.List
// 同名方法在单结构体上的语义冲突。
package sqlitestore

import (
	"database/sql"
	"fmt"
	"sync"

	_ "modernc.org/sqlite" // register sqlite driver
)

// Store 为 SQLite 存储核心，持有数据库句柄与并发锁。
type Store struct {
	db *sql.DB
	mu sync.Mutex // 保护 sends 裁剪与模板初始化等复合操作
}

// Open 打开（不存在则创建）SQLite 库文件并建表。
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q failed: %w", path, err)
	}
	db.SetMaxOpenConns(1) // SQLite 单写者，串行化避免锁竞争
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite %q failed: %w", path, err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close 关闭数据库。
func (s *Store) Close() error { return s.db.Close() }

// migrate 幂等建表：channels / templates / sends + sends 查询索引。
func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS channels (
			channel TEXT PRIMARY KEY,
			config  TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS templates (
			name    TEXT PRIMARY KEY,
			content TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sends (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp   INTEGER NOT NULL,
			channel     TEXT NOT NULL,
			kind        TEXT NOT NULL DEFAULT '',
			status      TEXT NOT NULL,
			error       TEXT NOT NULL DEFAULT '',
			alert_count INTEGER NOT NULL DEFAULT 0,
			duration_ms INTEGER NOT NULL DEFAULT 0,
			raw         TEXT NOT NULL DEFAULT '',
			title       TEXT NOT NULL DEFAULT '',
			text        TEXT NOT NULL DEFAULT '',
			markdown    TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sends_timestamp ON sends(timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_sends_channel ON sends(channel)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate failed: %w", err)
		}
	}
	return nil
}
