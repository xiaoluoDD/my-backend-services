package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// filePath 当前打开的数据库文件路径（绝对路径）。
var filePath string

// FilePath 返回已打开数据库的文件路径。
func FilePath() string {
	return filePath
}

// BackupToFile 将当前库一致性备份到 dest（VACUUM INTO，适用于 WAL 模式）。
func BackupToFile(db *sql.DB, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	abs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}

	escaped := strings.ReplaceAll(filepath.ToSlash(abs), "'", "''")
	_, err = db.Exec(`VACUUM INTO '` + escaped + `'`)
	if err != nil {
		return fmt.Errorf("vacuum into: %w", err)
	}
	return nil
}
