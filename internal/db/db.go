package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DefaultPath 默认数据库文件路径（相对进程工作目录）。
const DefaultPath = "data/wecom.db"

// Open 打开 SQLite 并执行迁移。
func Open(path string) (*sql.DB, error) {
	if path == "" {
		path = os.Getenv("DB_PATH")
	}
	if path == "" {
		path = DefaultPath
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := migrate(sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return sqlDB, nil
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS app_users (
			userid TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			mobile TEXT NOT NULL DEFAULT '',
			departments TEXT NOT NULL DEFAULT '',
			sources TEXT NOT NULL DEFAULT '',
			active INTEGER NOT NULL DEFAULT 1,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sync_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			started_at TEXT NOT NULL,
			finished_at TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			user_count INTEGER NOT NULL DEFAULT 0,
			error_message TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS corp_info (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_app_users_active ON app_users(active)`,
		`CREATE TABLE IF NOT EXISTS projects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			year TEXT NOT NULL DEFAULT '',
			work_no TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			manager_userid TEXT NOT NULL DEFAULT '',
			manager_name TEXT NOT NULL DEFAULT '',
			group_chat TEXT NOT NULL DEFAULT '',
			group_chat_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT '',
			start_date TEXT NOT NULL DEFAULT '',
			end_date TEXT NOT NULL DEFAULT '',
			tasks TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_projects_year ON projects(year)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return seedProjectsIfEmpty(db)
}
