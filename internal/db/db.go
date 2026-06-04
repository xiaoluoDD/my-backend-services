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

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	filePath = absPath

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
		`CREATE TABLE IF NOT EXISTS project_members (
			project_id INTEGER NOT NULL,
			userid TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (project_id, userid),
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_project_members_userid ON project_members(userid)`,
		`CREATE TABLE IF NOT EXISTS departments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE COLLATE NOCASE,
			updated_at TEXT NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	if err := ensureColumn(db, "app_users", "department_id", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	return seedProjectsIfEmpty(db)
}

func ensureColumn(db *sql.DB, table, column, def string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, def))
	if err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}
