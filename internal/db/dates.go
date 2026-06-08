package db

import "time"

// ParseDateOnly 解析 yyyy-MM-dd 或 yyyy-M-d 格式日期。
func ParseDateOnly(raw string) (time.Time, bool) {
	return parseDateOnly(raw)
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// ShouldRemindInWindow 判断 today 是否落在 [事件日-daysBefore, 事件日] 区间内（含首尾）。
// 例如 daysBefore=3 时，事件日前 3 天、前 2 天、前 1 天及当天均返回 true。
func ShouldRemindInWindow(eventDate string, daysBefore int, today time.Time) bool {
	event, ok := parseDateOnly(eventDate)
	if !ok {
		return false
	}
	if daysBefore < 0 {
		daysBefore = 0
	}
	windowStart := event.AddDate(0, 0, -daysBefore)
	todayDay := dateOnly(today)
	eventDay := dateOnly(event)
	return !todayDay.Before(windowStart) && !todayDay.After(eventDay)
}

// DaysUntilEvent 返回 today 到 eventDate 的天数（当天为 0，明天为 1）。
func DaysUntilEvent(eventDate string, today time.Time) (int, bool) {
	event, ok := parseDateOnly(eventDate)
	if !ok {
		return 0, false
	}
	todayDay := dateOnly(today)
	eventDay := dateOnly(event)
	return int(eventDay.Sub(todayDay).Hours() / 24), true
}
