package db

import (
	"database/sql"
	"time"
)

const (
	ReminderKindStart = "start"
	ReminderKindEnd   = "end"
)

// WasReminderSent 检查指定项目在某天是否已发送过某类提醒。
func WasReminderSent(db *sql.DB, projectID int64, kind, sentDate string) (bool, error) {
	if projectID <= 0 || kind == "" || sentDate == "" {
		return false, nil
	}
	var n int
	err := db.QueryRow(
		`SELECT COUNT(1) FROM reminder_sent WHERE project_id=? AND kind=? AND sent_date=?`,
		projectID, kind, sentDate,
	).Scan(&n)
	return n > 0, err
}

// RecordReminderSent 记录提醒已发送，避免同一天重复发送。
func RecordReminderSent(db *sql.DB, projectID int64, kind, sentDate string) error {
	if projectID <= 0 || kind == "" || sentDate == "" {
		return nil
	}
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO reminder_sent (project_id, kind, sent_date, sent_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(project_id, kind, sent_date) DO NOTHING`,
		projectID, kind, sentDate, now,
	)
	return err
}
