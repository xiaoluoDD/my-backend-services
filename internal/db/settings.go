package db

import (
	"database/sql"
	"strconv"
	"strings"
	"time"
)

const SettingServerBaseURL = "server_base_url"
const SettingReminderTime = "reminder_time"
const SettingProjectStartReminderDays = "project_start_reminder_days"
const SettingProjectEndReminderDays = "project_end_reminder_days"

const defaultProjectStartReminderDays = 1
const defaultProjectEndReminderDays = 3

// AppSettings 客户端可配置项。
type AppSettings struct {
	ServerBaseURL            string `json:"server_base_url"`
	ReminderTime             string `json:"reminder_time"`
	ProjectStartReminderDays int    `json:"project_start_reminder_days"`
	ProjectEndReminderDays   int    `json:"project_end_reminder_days"`
	UpdatedAt                string `json:"updated_at,omitempty"`
}

// GetAppSettings 读取应用设置（缺失项为空字符串）。
func GetAppSettings(db *sql.DB) (AppSettings, error) {
	m, err := listAppSettingsMap(db)
	if err != nil {
		return AppSettings{}, err
	}
	return AppSettings{
		ServerBaseURL:            m[SettingServerBaseURL],
		ReminderTime:             m[SettingReminderTime],
		ProjectStartReminderDays: parseReminderDays(m[SettingProjectStartReminderDays], defaultProjectStartReminderDays),
		ProjectEndReminderDays:   parseReminderDays(m[SettingProjectEndReminderDays], defaultProjectEndReminderDays),
		UpdatedAt:                m["_updated_at"],
	}, nil
}

// SaveAppSettings 保存应用设置（仅更新非空字段）。
func SaveAppSettings(db *sql.DB, s AppSettings) error {
	now := time.Now().Format(time.RFC3339)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if s.ServerBaseURL != "" {
		if err := upsertAppSetting(tx, SettingServerBaseURL, s.ServerBaseURL, now); err != nil {
			return err
		}
	}
	if err := upsertAppSetting(tx, SettingReminderTime, s.ReminderTime, now); err != nil {
		return err
	}
	if err := upsertAppSetting(tx, SettingProjectStartReminderDays, strconv.Itoa(NormalizeReminderDays(s.ProjectStartReminderDays)), now); err != nil {
		return err
	}
	if err := upsertAppSetting(tx, SettingProjectEndReminderDays, strconv.Itoa(NormalizeReminderDays(s.ProjectEndReminderDays)), now); err != nil {
		return err
	}
	return tx.Commit()
}

func parseReminderDays(raw string, defaultVal int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal
	}
	return NormalizeReminderDays(n)
}

// NormalizeReminderDays 将提前天数限制在 0–365。
func NormalizeReminderDays(n int) int {
	if n < 0 {
		return 0
	}
	if n > 365 {
		return 365
	}
	return n
}

func listAppSettingsMap(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query(`SELECT key, value, updated_at FROM app_settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]string)
	var latest string
	for rows.Next() {
		var k, v, updatedAt string
		if err := rows.Scan(&k, &v, &updatedAt); err != nil {
			return nil, err
		}
		out[k] = v
		if updatedAt > latest {
			latest = updatedAt
		}
	}
	if latest != "" {
		out["_updated_at"] = latest
	}
	return out, rows.Err()
}

func upsertAppSetting(tx *sql.Tx, key, value, now string) error {
	_, err := tx.Exec(
		`INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		key, value, now,
	)
	return err
}
