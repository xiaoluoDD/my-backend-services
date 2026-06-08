package db

import (
	"database/sql"
	"time"
)

const (
	ReminderKindStart = "start"
	ReminderKindEnd   = "end"

	// 子任务计划开始摘要（方案 C）。
	ReminderKindSubtaskStartDigestMgr    = "subtask_start_digest_mgr"
	ReminderKindSubtaskStartDigestMember = "subtask_start_digest_member"
	ReminderKindSubtaskStart             = "subtask_start"
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

// RecordReminderSent 记录项目级提醒已发送。
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

// WasSubtaskReminderSent 检查子任务在某天是否已纳入提醒。
func WasSubtaskReminderSent(db *sql.DB, subtaskID int64, kind, sentDate string) (bool, error) {
	if subtaskID <= 0 || kind == "" || sentDate == "" {
		return false, nil
	}
	var n int
	err := db.QueryRow(
		`SELECT COUNT(1) FROM subtask_reminder_sent WHERE subtask_id=? AND kind=? AND sent_date=?`,
		subtaskID, kind, sentDate,
	).Scan(&n)
	return n > 0, err
}

// RecordSubtaskReminderSent 记录子任务已提醒。
func RecordSubtaskReminderSent(db *sql.DB, subtaskID int64, kind, sentDate string) error {
	if subtaskID <= 0 || kind == "" || sentDate == "" {
		return nil
	}
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO subtask_reminder_sent (subtask_id, kind, sent_date, sent_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(subtask_id, kind, sent_date) DO NOTHING`,
		subtaskID, kind, sentDate, now,
	)
	return err
}

// WasUserReminderSent 检查用户在某天是否已收到某类摘要提醒。
func WasUserReminderSent(db *sql.DB, userID, kind, sentDate string) (bool, error) {
	if userID == "" || kind == "" || sentDate == "" {
		return false, nil
	}
	var n int
	err := db.QueryRow(
		`SELECT COUNT(1) FROM user_reminder_sent WHERE userid=? AND kind=? AND sent_date=?`,
		userID, kind, sentDate,
	).Scan(&n)
	return n > 0, err
}

// RecordUserReminderSent 记录用户级摘要已发送。
func RecordUserReminderSent(db *sql.DB, userID, kind, sentDate string) error {
	if userID == "" || kind == "" || sentDate == "" {
		return nil
	}
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO user_reminder_sent (userid, kind, sent_date, sent_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(userid, kind, sent_date) DO NOTHING`,
		userID, kind, sentDate, now,
	)
	return err
}
