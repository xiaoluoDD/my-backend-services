package db

import (
	"database/sql"
	"strings"
	"time"
)

const SettingDeepseekAPIKey = "deepseek_api_key"

// GetDeepseekAPIKey 读取已保存的 DeepSeek 密钥。未绑定返回空字符串。
func GetDeepseekAPIKey(db *sql.DB) (string, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, SettingDeepseekAPIKey).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

// SaveDeepseekAPIKey 保存密钥。空字符串表示清除绑定。
func SaveDeepseekAPIKey(db *sql.DB, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		_, err := db.Exec(`DELETE FROM app_settings WHERE key = ?`, SettingDeepseekAPIKey)
		return err
	}
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		SettingDeepseekAPIKey, key, now,
	)
	return err
}

// MaskSecret 给管理界面显示用，不返回完整密钥。
func MaskSecret(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	r := []rune(secret)
	if len(r) <= 8 {
		return "已绑定"
	}
	return string(r[:3]) + "****" + string(r[len(r)-4:])
}
