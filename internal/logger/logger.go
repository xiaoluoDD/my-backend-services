// Package logger 提供统一结构化日志（控制台 + 可选文件）。
package logger

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	mu      sync.Mutex
	defaultLogger *slog.Logger = slog.Default()
)

// Init 初始化日志。component 用于日志文件名，如 http、wecom、tcp。
func Init(component string) *slog.Logger {
	mu.Lock()
	defer mu.Unlock()

	level := parseLevel(os.Getenv("LOG_LEVEL"))
	writers := []io.Writer{os.Stdout}

	if dir := logDir(); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
		name := component + ".log"
		if strings.EqualFold(os.Getenv("LOG_DAILY"), "true") || os.Getenv("LOG_DAILY") == "1" {
			name = component + "-" + time.Now().Format("2006-01-02") + ".log"
		}
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			slog.Default().Error("open log file failed", "path", path, "err", err)
		} else {
			writers = append(writers, f)
		}
	}

	h := slog.NewTextHandler(io.MultiWriter(writers...), &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	})
	l := slog.New(h).With("service", component)
	defaultLogger = l
	slog.SetDefault(l)
	return l
}

// Default 返回已初始化的 Logger；未 Init 时为标准库默认 Logger。
func Default() *slog.Logger {
	mu.Lock()
	defer mu.Unlock()
	return defaultLogger
}

func logDir() string {
	v, ok := os.LookupEnv("LOG_DIR")
	if ok {
		return strings.TrimSpace(v)
	}
	return "logs"
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// HTTPMiddleware 记录每个 HTTP 请求的方法、路径、状态码与耗时。
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"status", sw.status,
			"bytes", sw.bytes,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		}
		if ua := r.UserAgent(); ua != "" {
			attrs = append(attrs, "user_agent", ua)
		}

		l := Default()
		if sw.status >= 500 {
			l.Error("http request", attrs...)
		} else if sw.status >= 400 {
			l.Warn("http request", attrs...)
		} else {
			l.Info("http request", attrs...)
		}
	})
}
