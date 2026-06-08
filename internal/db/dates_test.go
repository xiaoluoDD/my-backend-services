package db

import (
	"testing"
	"time"
)

func TestShouldRemindInWindow_DayBeforeStartInLocalTZ(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skip("Asia/Shanghai not available")
	}
	// 启动日 6/9，提前 1 天 → 6/8 应在窗口内（CST 下午任意时刻）。
	today := time.Date(2026, 6, 8, 15, 37, 0, 0, loc)
	if !ShouldRemindInWindow("2026-06-09", 1, today) {
		t.Fatal("expected 2026-06-08 to be inside reminder window for start 2026-06-09 with 1 day before")
	}
}

func TestShouldRemindInWindow_OutsideWindow(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skip("Asia/Shanghai not available")
	}
	today := time.Date(2026, 6, 7, 10, 0, 0, 0, loc)
	if ShouldRemindInWindow("2026-06-09", 1, today) {
		t.Fatal("expected 2026-06-07 to be outside 1-day window ending 2026-06-09")
	}
}
