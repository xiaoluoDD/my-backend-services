package reminder

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
	"github.com/xiaoluoDD/my-backend-services/internal/wecom"
)

// RunResult 单次提醒任务执行结果。
type RunResult struct {
	StartSent int      `json:"start_sent"`
	EndSent   int      `json:"end_sent"` // 已停用，恒为 0
	Skipped   int      `json:"skipped"`
	Errors    []string `json:"errors,omitempty"`
	RunDate   string   `json:"run_date"`
}

// RunDaily 扫描全部项目，按设置向负责人发送启动前提醒。
func RunDaily(sqlDB *sql.DB, settings db.AppSettings) RunResult {
	result := RunResult{RunDate: time.Now().Format("2006-01-02")}

	projects, err := db.ListProjects(sqlDB)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}

	today := time.Now()
	startDays := db.NormalizeReminderDays(settings.ProjectStartReminderDays)

	for _, project := range projects {
		plannedStart := project.StartDate

		if !shouldSendStartReminder(project, plannedStart, startDays, today) {
			continue
		}

		sent, err := sendStartIfNotSent(sqlDB, project, startDays, plannedStart, today)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("项目 %d 启动提醒: %v", project.ID, err))
		} else if sent {
			result.StartSent++
		} else {
			result.Skipped++
		}
	}

	slog.Info("reminder · finished",
		"date", result.RunDate,
		"projects", len(projects),
		"start", result.StartSent,
		"skipped", result.Skipped,
		"errors", len(result.Errors),
	)
	return result
}

func shouldSendStartReminder(p db.Project, plannedStart string, daysBefore int, today time.Time) bool {
	if plannedStart == "" {
		return false
	}
	if db.EffectiveProjectStatus(p) != db.ProjectStatusNotStarted {
		return false
	}
	return db.ShouldRemindInWindow(plannedStart, daysBefore, today)
}

func sendStartIfNotSent(
	sqlDB *sql.DB,
	project db.Project,
	daysBefore int,
	eventDate string,
	today time.Time,
) (bool, error) {
	sentDate := today.Format("2006-01-02")
	if oncePerDayReminderDedup {
		already, err := db.WasReminderSent(sqlDB, project.ID, db.ReminderKindStart, sentDate)
		if err != nil {
			return false, err
		}
		if already {
			return false, nil
		}
	}

	extra := wecom.FormatScheduledReminderHeader(db.ReminderKindStart, daysRemaining(eventDate, today), eventDate)
	msgID, _, _, err := wecom.NotifyProjectManagerEx(project, db.ProjectSubtaskStats{}, extra)
	if err != nil {
		return false, err
	}

	if oncePerDayReminderDedup {
		if err := db.RecordReminderSent(sqlDB, project.ID, db.ReminderKindStart, sentDate); err != nil {
			slog.Warn("reminder · record failed", "project_id", project.ID, "kind", db.ReminderKindStart, "err", err)
		}
	}

	slog.Info("reminder · sent",
		"project_id", project.ID,
		"name", project.Name,
		"kind", db.ReminderKindStart,
		"msgid", msgID,
		"days_left", daysRemaining(eventDate, today),
		"event", eventDate,
	)
	return true, nil
}

func daysRemaining(eventDate string, today time.Time) int {
	d, ok := db.DaysUntilEvent(eventDate, today)
	if !ok || d < 0 {
		return 0
	}
	return d
}
