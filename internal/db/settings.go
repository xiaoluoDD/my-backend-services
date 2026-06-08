package db

import (
	"database/sql"
	"time"
)

const SettingServerBaseURL = "server_base_url"

// AppSettings 客户端可配置项。
type AppSettings struct {
	ServerBaseURL string `json:"server_base_url"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

// GetAppSettings 读取应用设置（缺失项为空字符串）。
func GetAppSettings(db *sql.DB) (AppSettings, error) {
	m, err := listAppSettingsMap(db)
	if err != nil {
		return AppSettings{}, err
	}
	return AppSettings{
		ServerBaseURL: m[SettingServerBaseURL],
		UpdatedAt:     m["_updated_at"],
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
	return tx.Commit()
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
