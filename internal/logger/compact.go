package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

// compactHandler 输出便于人工浏览的单行日志。
// 示例：2026-06-08 13:40:00 INF [http] reminder · finished  start=0 end=0 skipped=3
type compactHandler struct {
	w       io.Writer
	level   slog.Level
	service string
	mu      sync.Mutex
}

func newCompactHandler(w io.Writer, level slog.Level, service string) *compactHandler {
	return &compactHandler{w: w, level: level, service: service}
}

func (h *compactHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *compactHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Time.Format("2006-01-02 15:04:05"))
	b.WriteByte(' ')
	b.WriteString(levelLabel(r.Level))
	if h.service != "" {
		b.WriteString(" [")
		b.WriteString(h.service)
		b.WriteByte(']')
	}
	b.WriteByte(' ')
	b.WriteString(r.Message)

	r.Attrs(func(a slog.Attr) bool {
		if a.Equal(slog.Attr{}) {
			return true
		}
		if a.Key == "service" && a.Value.String() == h.service {
			return true
		}
		b.WriteByte(' ')
		b.WriteString(formatAttr(a))
		return true
	})

	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write([]byte(b.String()))
	return err
}

func (h *compactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	return &withAttrsHandler{parent: h, attrs: attrs}
}

func (h *compactHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &withGroupHandler{parent: h, group: name}
}

type withAttrsHandler struct {
	parent slog.Handler
	attrs  []slog.Attr
}

func (h *withAttrsHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.parent.Enabled(ctx, level)
}

func (h *withAttrsHandler) Handle(ctx context.Context, r slog.Record) error {
	r.AddAttrs(h.attrs...)
	return h.parent.Handle(ctx, r)
}

func (h *withAttrsHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.attrs = append(h.attrs, attrs...)
	return h
}

func (h *withAttrsHandler) WithGroup(name string) slog.Handler {
	return h.parent.WithGroup(name)
}

type withGroupHandler struct {
	parent slog.Handler
	group  string
}

func (h *withGroupHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.parent.Enabled(ctx, level)
}

func (h *withGroupHandler) Handle(ctx context.Context, r slog.Record) error {
	r.AddAttrs(slog.String("group", h.group))
	return h.parent.Handle(ctx, r)
}

func (h *withGroupHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h.parent.WithAttrs(attrs)
}

func (h *withGroupHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &withGroupHandler{parent: h.parent, group: h.group + "." + name}
}

func levelLabel(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "ERR"
	case l >= slog.LevelWarn:
		return "WRN"
	case l >= slog.LevelInfo:
		return "INF"
	default:
		return "DBG"
	}
}

func formatAttr(a slog.Attr) string {
	a.Value = a.Value.Resolve()
	switch a.Value.Kind() {
	case slog.KindString:
		v := a.Value.String()
		if strings.ContainsAny(v, " \t=") {
			return fmt.Sprintf("%s=%q", a.Key, v)
		}
		return a.Key + "=" + v
	case slog.KindInt64:
		return fmt.Sprintf("%s=%d", a.Key, a.Value.Int64())
	case slog.KindUint64:
		return fmt.Sprintf("%s=%d", a.Key, a.Value.Uint64())
	case slog.KindFloat64:
		return fmt.Sprintf("%s=%g", a.Key, a.Value.Float64())
	case slog.KindBool:
		return fmt.Sprintf("%s=%t", a.Key, a.Value.Bool())
	case slog.KindDuration:
		return fmt.Sprintf("%s=%s", a.Key, a.Value.String())
	default:
		return fmt.Sprintf("%s=%v", a.Key, a.Value.Any())
	}
}
