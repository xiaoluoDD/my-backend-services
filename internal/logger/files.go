package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FileInfo 日志文件元信息。
type FileInfo struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
}

// LogDir 返回当前日志目录（相对或绝对路径）。
func LogDir() string {
	return logDir()
}

// ListLogFiles 列出日志目录下的 .log 文件。
func ListLogFiles() ([]FileInfo, error) {
	dir := logDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, FileInfo{
			Name:       e.Name(),
			Size:       info.Size(),
			ModifiedAt: info.ModTime().Format(time.RFC3339),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModifiedAt > files[j].ModifiedAt
	})
	return files, nil
}

// ResolveLogFile 校验文件名并返回绝对路径（防止目录穿越）。
func ResolveLogFile(name string) (string, error) {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." || strings.Contains(base, "..") {
		return "", fmt.Errorf("非法日志文件名")
	}
	if !strings.HasSuffix(strings.ToLower(base), ".log") {
		return "", fmt.Errorf("仅支持 .log 文件")
	}

	dir, err := filepath.Abs(logDir())
	if err != nil {
		return "", err
	}
	full := filepath.Join(dir, base)
	abs, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(dir, abs)
	if err != nil || strings.HasPrefix(rel, "..") || strings.Contains(rel, "..") {
		return "", fmt.Errorf("非法路径")
	}
	return abs, nil
}
