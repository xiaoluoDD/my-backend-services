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

// StartScheduler 在后台按 app_settings.reminder_time 每分钟检查并触发提醒扫描。
func StartScheduler(sqlDB *sql.DB) {
	go func() {
		logSchedulerStarted(sqlDB)
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

func logSchedulerStarted(sqlDB *sql.DB) {
	settings, err := db.GetAppSettings(sqlDB)
	reminderTime := ""
	if err != nil {
		slog.Warn("reminder · scheduler started (settings unreadable)", "err", err, "tz", time.Now().Format("MST"))
		return
	}
	reminderTime = settings.ReminderTime
	if reminderTime == "" {
		slog.Warn("reminder · scheduler started", "reminder_time", "unset", "tz", time.Now().Format("MST"))
		return
	}
	slog.Info("reminder · scheduler started",
		"reminder_time", reminderTime,
		"tz", time.Now().Format("MST"),
		"start_days", settings.StartReminderDays(),
		"end_days", settings.EndReminderDays(),
	)
}

func waitUntilNextMinute() {
	now := time.Now()
	next := now.Truncate(time.Minute).Add(time.Minute)
	time.Sleep(time.Until(next))
}

func maybeRunDaily(sqlDB *sql.DB) {
	settings, err := db.GetAppSettings(sqlDB)
	if err != nil {
		slog.Error("reminder · load settings failed", "err", err)
		return
	}

	reminderTime := settings.ReminderTime
	now := time.Now()
	nowHM := now.Format("15:04")

	if reminderTime == "" {
		if schedulerMinuteLog {
			slog.Info("reminder · tick", "now", nowHM, "reminder_time", "unset", "action", "skip")
		}
		return
	}

	matched := nowHM == reminderTime
	if schedulerMinuteLog {
		action := "wait"
		if matched {
			action = "trigger"
		}
		slog.Info("reminder · tick", "now", nowHM, "reminder_time", reminderTime, "action", action)
	}

	if !matched {
		return
	}

	if oncePerDayReminderRun {
		today := now.Format("2006-01-02")
		runMu.Lock()
		if lastRunDate == today {
			runMu.Unlock()
			slog.Info("reminder · skip", "reason", "already_ran_today", "date", today)
			return
		}
		lastRunDate = today
		runMu.Unlock()
	}

	slog.Info("reminder · trigger", "time", reminderTime)
	RunDaily(sqlDB, settings)
}

// LogSettingsUpdated 保存提醒相关设置后输出一条日志，便于对照 tick。
func LogSettingsUpdated(settings db.AppSettings) {
	if settings.ReminderTime == "" {
		slog.Warn("reminder · settings updated", "reminder_time", "unset")
		return
	}
	slog.Info("reminder · settings updated",
		"reminder_time", settings.ReminderTime,
		"start_days", settings.StartReminderDays(),
		"end_days", settings.EndReminderDays(),
		"server_now", time.Now().Format("15:04"),
		"tz", time.Now().Format("MST"),
	)
}
