package reminder

import (
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
)

var (
	runMu       sync.Mutex
	lastRunDate string
)

// oncePerDayReminderRun 定版后改为 true：同一自然日仅在 reminder_time 对应分钟触发一次。
// 调试阶段设为 false，改提醒时间后当天可再次触发。
const oncePerDayReminderRun = false

// StartScheduler 在后台按 app_settings.reminder_time 每分钟检查并触发提醒扫描。
func StartScheduler(sqlDB *sql.DB) {
	go func() {
		slog.Info("reminder scheduler started")
		// 启动后先对齐到下一分钟，再每分钟检查一次。
		waitUntilNextMinute()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			maybeRunDaily(sqlDB)
			<-ticker.C
		}
	}()
}

func waitUntilNextMinute() {
	now := time.Now()
	next := now.Truncate(time.Minute).Add(time.Minute)
	time.Sleep(time.Until(next))
}

func maybeRunDaily(sqlDB *sql.DB) {
	settings, err := db.GetAppSettings(sqlDB)
	if err != nil {
		slog.Error("reminder scheduler load settings failed", "err", err)
		return
	}

	reminderTime := settings.ReminderTime
	if reminderTime == "" {
		return
	}

	now := time.Now()
	if now.Format("15:04") != reminderTime {
		return
	}

	if oncePerDayReminderRun {
		today := now.Format("2006-01-02")
		runMu.Lock()
		if lastRunDate == today {
			runMu.Unlock()
			return
		}
		lastRunDate = today
		runMu.Unlock()
	}

	slog.Info("reminder scheduler triggering daily run", "reminder_time", reminderTime)
	RunDaily(sqlDB, settings)
}
