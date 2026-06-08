package db

import "time"

// ParseDateOnly 解析 yyyy-MM-dd 或 yyyy-M-d 格式日期。
func ParseDateOnly(raw string) (time.Time, bool) {
	return parseDateOnly(raw)
}

// ShouldRemindOnDate 判断 today 是否为「事件日 - daysBefore」的提醒日。
func ShouldRemindOnDate(eventDate string, daysBefore int, today time.Time) bool {
	event, ok := parseDateOnly(eventDate)
	if !ok {
		return false
	}
	if daysBefore < 0 {
		daysBefore = 0
	}
	remindDay := event.AddDate(0, 0, -daysBefore)
	todayDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	return remindDay.Equal(todayDay)
}
