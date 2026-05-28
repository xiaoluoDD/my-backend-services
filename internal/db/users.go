package db

import (
	"database/sql"
	"fmt"
	"time"
)

// AppUser 应用可见范围内的成员（持久化）。
type AppUser struct {
	UserID      string `json:"userid"`
	Name        string `json:"name"`
	Mobile      string `json:"mobile,omitempty"`
	Departments string `json:"departments"`
	Sources     string `json:"sources"`
	UpdatedAt   string `json:"updated_at"`
}

// SyncRun 一次同步任务记录。
type SyncRun struct {
	ID           int64  `json:"id"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at,omitempty"`
	Status       string `json:"status"`
	UserCount    int    `json:"user_count"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// UpsertCorpInfo 写入企业/应用配置快照。
func UpsertCorpInfo(db *sql.DB, pairs map[string]string) error {
	now := time.Now().Format(time.RFC3339)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for k, v := range pairs {
		_, err := tx.Exec(
			`INSERT INTO corp_info (key, value, updated_at) VALUES (?, ?, ?)
			 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
			k, v, now,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListCorpInfo 读取企业配置快照。
func ListCorpInfo(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query(`SELECT key, value FROM corp_info ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// BeginSync 创建同步任务记录，返回 run ID。
func BeginSync(db *sql.DB) (int64, error) {
	now := time.Now().Format(time.RFC3339)
	res, err := db.Exec(
		`INSERT INTO sync_runs (started_at, status) VALUES (?, 'running')`,
		now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// FinishSync 结束同步任务。
func FinishSync(db *sql.DB, runID int64, status string, userCount int, errMsg string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(
		`UPDATE sync_runs SET finished_at=?, status=?, user_count=?, error_message=? WHERE id=?`,
		now, status, userCount, errMsg, runID,
	)
	return err
}

// LastSyncRun 返回最近一次同步记录。
func LastSyncRun(db *sql.DB) (*SyncRun, error) {
	row := db.QueryRow(
		`SELECT id, started_at, finished_at, status, user_count, error_message
		 FROM sync_runs ORDER BY id DESC LIMIT 1`,
	)
	var s SyncRun
	if err := row.Scan(&s.ID, &s.StartedAt, &s.FinishedAt, &s.Status, &s.UserCount, &s.ErrorMessage); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// ReplaceAppUsers 全量替换当前可见成员（事务）。
func ReplaceAppUsers(db *sql.DB, users []AppUser) error {
	now := time.Now().Format(time.RFC3339)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE app_users SET active=0`); err != nil {
		return err
	}

	stmt, err := tx.Prepare(
		`INSERT INTO app_users (userid, name, mobile, departments, sources, active, updated_at)
		 VALUES (?, ?, ?, ?, ?, 1, ?)
		 ON CONFLICT(userid) DO UPDATE SET
		   name=excluded.name,
		   mobile=excluded.mobile,
		   departments=excluded.departments,
		   sources=excluded.sources,
		   active=1,
		   updated_at=excluded.updated_at`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, u := range users {
		updated := u.UpdatedAt
		if updated == "" {
			updated = now
		}
		if _, err := stmt.Exec(u.UserID, u.Name, u.Mobile, u.Departments, u.Sources, updated); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListActiveUsers 列出当前有效成员。
func ListActiveUsers(db *sql.DB) ([]AppUser, error) {
	rows, err := db.Query(
		`SELECT userid, name, mobile, departments, sources, updated_at
		 FROM app_users WHERE active=1 ORDER BY name, userid`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AppUser
	for rows.Next() {
		var u AppUser
		if err := rows.Scan(&u.UserID, &u.Name, &u.Mobile, &u.Departments, &u.Sources, &u.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, u)
	}
	return list, rows.Err()
}

// CountActiveUsers 统计有效成员数。
func CountActiveUsers(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM app_users WHERE active=1`).Scan(&n)
	return n, err
}

// Stats 汇总数据库状态。
func Stats(db *sql.DB) (map[string]interface{}, error) {
	count, err := CountActiveUsers(db)
	if err != nil {
		return nil, err
	}
	last, err := LastSyncRun(db)
	if err != nil {
		return nil, err
	}
	corp, err := ListCorpInfo(db)
	if err != nil {
		return nil, err
	}

	out := map[string]interface{}{
		"active_users": count,
		"corp_info":    corp,
	}
	if last != nil {
		out["last_sync"] = last
	}
	return out, nil
}

// FormatSyncError 包装同步错误信息。
func FormatSyncError(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%v", err)
}
