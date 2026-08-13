package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// AccountLoginLog 账户登录记录。
type AccountLoginLog struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	LoggedAt string `json:"logged_at"`
}

// RecordAccountLogin 写入一次成功登录时间。
func RecordAccountLogin(dbConn *sql.DB, username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("用户名为空")
	}
	now := time.Now().Format(time.RFC3339)
	_, err := dbConn.Exec(
		`INSERT INTO account_login_logs (username, logged_at) VALUES (?, ?)`,
		username, now,
	)
	return err
}

// ListRecentAccountLogins 返回指定用户最近 limit 次登录（按时间倒序）。
func ListRecentAccountLogins(dbConn *sql.DB, username string, limit int) ([]AccountLoginLog, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("请提供用户名")
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := dbConn.Query(
		`SELECT id, username, logged_at
		 FROM account_login_logs
		 WHERE username=? COLLATE NOCASE
		 ORDER BY logged_at DESC, id DESC
		 LIMIT ?`,
		username, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]AccountLoginLog, 0, limit)
	for rows.Next() {
		var item AccountLoginLog
		if err := rows.Scan(&item.ID, &item.Username, &item.LoggedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

// DeleteAccountLoginLogs 删除某用户的全部登录记录。
func DeleteAccountLoginLogs(dbConn *sql.DB, username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil
	}
	_, err := dbConn.Exec(`DELETE FROM account_login_logs WHERE username=? COLLATE NOCASE`, username)
	return err
}
