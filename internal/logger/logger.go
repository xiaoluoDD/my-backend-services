// Package logger 提供统一结构化日志（控制台 + 可选文件）。
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	mu            sync.Mutex
	defaultLogger *slog.Logger = slog.Default()
	logFileWriter io.WriteCloser
)

// Init 初始化日志。component 用于日志文件名，如 http、wecom、tcp。
// 默认按日写入 logs/{component}-YYYY-MM-DD.log；LOG_DAILY=false 时写入单文件 {component}.log。
// LOG_RETENTION_DAYS 默认 30，启动时删除更早的 .log 文件。
// 环境变量 LOG_FORMAT：compact（默认，便于浏览）、text（slog 原始格式）、json。
func Init(component string) *slog.Logger {
	mu.Lock()
	defer mu.Unlock()

	if logFileWriter != nil {
		_ = logFileWriter.Close()
		logFileWriter = nil
	}

	level := parseLevel(os.Getenv("LOG_LEVEL"))
	writers := []io.Writer{os.Stdout}

	if dir := logDir(); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
		cleanupOldLogFiles(dir, logRetentionDays())

		if dailyLogEnabled() {
			w, err := newDailyFileWriter(dir, component)
			if err != nil {
				slog.Default().Error("open daily log file failed", "dir", dir, "component", component, "err", err)
			} else {
				logFileWriter = w
				writers = append(writers, w)
			}
		} else {
			path := filepath.Join(dir, component+".log")
			f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				slog.Default().Error("open log file failed", "path", path, "err", err)
			} else {
				logFileWriter = f
				writers = append(writers, f)
			}
		}
	}

	multi := io.MultiWriter(writers...)
	l := newLogger(multi, level, component, logFormat())
	defaultLogger = l
	slog.SetDefault(l)
	return l
}

func logFormat() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT"))) {
	case "text", "json":
		return strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT")))
	default:
		return "compact"
	}
}

func newLogger(w io.Writer, level slog.Level, component, format string) *slog.Logger {
	switch format {
	case "json":
		h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
		return slog.New(h).With("service", component)
	case "text":
		h := slog.NewTextHandler(w, &slog.HandlerOptions{
			Level:     level,
			AddSource: level == slog.LevelDebug,
		})
		return slog.New(h).With("service", component)
	default:
		h := newCompactHandler(w, level, component)
		return slog.New(h)
	}
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

func httpLogLevel(r *http.Request, status int) slog.Level {
	if status >= 500 {
		return slog.LevelError
	}
	if status >= 400 {
		return slog.LevelWarn
	}
	if shouldHTTPDebug(r) {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}

func shouldHTTPDebug(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	path := r.URL.Path
	switch path {
	case "/ping", "/api/logs":
		return true
	}
	if strings.HasPrefix(path, "/api/logs/") {
		return true
	}
	return false
}

func shouldSkipHTTPLog(r *http.Request) bool {
	return r.URL.Path == "/ping"
}

func formatHTTPTarget(r *http.Request) string {
	target := r.URL.Path
	if q := r.URL.RawQuery; q != "" {
		target += "?" + q
	}
	return target
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func formatByteSize(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(n)/(1<<10))
	case n > 0:
		return fmt.Sprintf("%dB", n)
	default:
		return ""
	}
}

// HTTPMiddleware 记录每个 HTTP 请求（紧凑单行，健康检查与日志接口降级为 DEBUG）。
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		if shouldSkipHTTPLog(r) {
			return
		}

		ms := time.Since(start).Milliseconds()
		target := formatHTTPTarget(r)
		size := formatByteSize(sw.bytes)
		ip := clientIP(r.RemoteAddr)

		msg := fmt.Sprintf("%s %s → %d %dms", r.Method, target, sw.status, ms)
		if size != "" {
			msg += " " + size
		}
		if ip != "" {
			msg += " " + ip
		}

		l := Default()
		switch httpLogLevel(r, sw.status) {
		case slog.LevelError:
			l.Error(msg)
		case slog.LevelWarn:
			l.Warn(msg)
		case slog.LevelDebug:
			l.Debug(msg)
		default:
			l.Info(msg)
		}
	})
}
