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
	EndSent   int      `json:"end_sent"`
	Skipped   int      `json:"skipped"`
	Errors    []string `json:"errors,omitempty"`
	RunDate   string   `json:"run_date"`
}

// RunDaily 扫描全部项目，按设置发送启动/完结前提醒。
func RunDaily(sqlDB *sql.DB, settings db.AppSettings) RunResult {
	result := RunResult{RunDate: time.Now().Format("2006-01-02")}

	projects, err := db.ListProjects(sqlDB)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}

	statsMap, err := db.SummarizeAllProjectSubtaskStats(sqlDB)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}

	today := time.Now()
	startDays := db.NormalizeReminderDays(settings.ProjectStartReminderDays)
	endDays := db.NormalizeReminderDays(settings.ProjectEndReminderDays)

	for _, project := range projects {
		stats := statsMap[project.ID]
		members, err := db.ListProjectMembers(sqlDB, project.ID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("项目 %d 读取成员失败: %v", project.ID, err))
			continue
		}

		plannedStart := pickPlannedStart(project)
		plannedEnd := pickPlannedEnd(stats)

		if shouldSendStartReminder(project, plannedStart, startDays, today) {
			sent, err := sendIfNotSent(sqlDB, project, members, stats, db.ReminderKindStart, startDays, plannedStart, today)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("项目 %d 启动提醒: %v", project.ID, err))
			} else if sent {
				result.StartSent++
			} else {
				result.Skipped++
			}
		}

		if shouldSendEndReminder(project, plannedEnd, endDays, today) {
			sent, err := sendIfNotSent(sqlDB, project, members, stats, db.ReminderKindEnd, endDays, plannedEnd, today)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("项目 %d 完结提醒: %v", project.ID, err))
			} else if sent {
				result.EndSent++
			} else {
				result.Skipped++
			}
		}
	}

	slog.Info("reminder · finished",
		"date", result.RunDate,
		"start", result.StartSent,
		"end", result.EndSent,
		"skipped", result.Skipped,
		"errors", len(result.Errors),
	)
	return result
}

func pickPlannedStart(p db.Project) string {
	return p.StartDate
}

func pickPlannedEnd(stats db.ProjectSubtaskStats) string {
	return stats.SubtaskEndDate
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

func shouldSendEndReminder(p db.Project, plannedEnd string, daysBefore int, today time.Time) bool {
	if plannedEnd == "" {
		return false
	}
	if db.ProjectHasActualEnd(p) {
		return false
	}
	return db.ShouldRemindInWindow(plannedEnd, daysBefore, today)
}

func sendIfNotSent(
	sqlDB *sql.DB,
	project db.Project,
	members []db.ProjectMember,
	stats db.ProjectSubtaskStats,
	kind string,
	daysBefore int,
	eventDate string,
	today time.Time,
) (bool, error) {
	sentDate := today.Format("2006-01-02")
	if oncePerDayReminderDedup {
		already, err := db.WasReminderSent(sqlDB, project.ID, kind, sentDate)
		if err != nil {
			return false, err
		}
		if already {
			return false, nil
		}
	}

	extra := wecom.FormatScheduledReminderHeader(kind, daysRemaining(eventDate, today), eventDate)
	msgID, _, _, err := wecom.NotifyProjectMembersEx(project, members, stats, extra)
	if err != nil {
		return false, err
	}

	if oncePerDayReminderDedup {
		if err := db.RecordReminderSent(sqlDB, project.ID, kind, sentDate); err != nil {
			slog.Warn("reminder · record failed", "project_id", project.ID, "kind", kind, "err", err)
		}
	}

	slog.Info("reminder · sent",
		"project_id", project.ID,
		"name", project.Name,
		"kind", kind,
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
