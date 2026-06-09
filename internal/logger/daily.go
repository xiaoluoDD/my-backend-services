package logger

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var logDateSuffix = regexp.MustCompile(`-(\d{4}-\d{2}-\d{2})\.log$`)

// dailyFileWriter 按本地日历日写入 component-YYYY-MM-DD.log，跨天自动切换文件。
type dailyFileWriter struct {
	dir       string
	component string
	mu        sync.Mutex
	curDate   string
	file      *os.File
}

func newDailyFileWriter(dir, component string) (*dailyFileWriter, error) {
	w := &dailyFileWriter{dir: dir, component: component}
	if err := w.rotateIfNeeded(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *dailyFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.rotateIfNeeded(); err != nil {
		return 0, err
	}
	return w.file.Write(p)
}

func (w *dailyFileWriter) rotateIfNeeded() error {
	today := time.Now().Format("2006-01-02")
	if w.file != nil && w.curDate == today {
		return nil
	}
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	w.curDate = today
	path := filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.component, today))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w.file = f
	return nil
}

func (w *dailyFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func dailyLogEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_DAILY")))
	if v == "false" || v == "0" {
		return false
	}
	return true
}

func logRetentionDays() int {
	raw := strings.TrimSpace(os.Getenv("LOG_RETENTION_DAYS"))
	if raw == "" {
		return 30
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 30
	}
	return n
}

func cleanupOldLogFiles(dir string, retentionDays int) {
	if retentionDays <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Default().Warn("cleanup log dir read failed", "dir", dir, "err", err)
		return
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".log") {
			continue
		}

		remove := false
		if m := logDateSuffix.FindStringSubmatch(name); len(m) == 2 {
			if t, err := time.ParseInLocation("2006-01-02", m[1], time.Local); err == nil {
				remove = t.Before(cutoff)
			}
		}
		if !remove {
			info, err := e.Info()
			if err != nil {
				continue
			}
			remove = info.ModTime().Before(cutoff)
		}
		if !remove {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			slog.Default().Warn("cleanup log file failed", "name", name, "err", err)
		} else {
			slog.Default().Info("cleanup log file removed", "name", name)
		}
	}
}
