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

		plannedStart := pickPlannedStart(project, stats)
		plannedEnd := pickPlannedEnd(project, stats)

		if shouldSendStartReminder(plannedStart, startDays, stats, today) {
			sent, err := sendIfNotSent(sqlDB, project, members, stats, db.ReminderKindStart, startDays, plannedStart, today)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("项目 %d 启动提醒: %v", project.ID, err))
			} else if sent {
				result.StartSent++
			} else {
				result.Skipped++
			}
		}

		if shouldSendEndReminder(plannedEnd, endDays, stats, today) {
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

	slog.Info("daily reminder finished",
		"run_date", result.RunDate,
		"start_sent", result.StartSent,
		"end_sent", result.EndSent,
		"skipped", result.Skipped,
		"errors", len(result.Errors),
	)
	return result
}

func pickPlannedStart(p db.Project, stats db.ProjectSubtaskStats) string {
	if stats.SubtaskStartDate != "" {
		return stats.SubtaskStartDate
	}
	return p.StartDate
}

func pickPlannedEnd(p db.Project, stats db.ProjectSubtaskStats) string {
	if stats.SubtaskEndDate != "" {
		return stats.SubtaskEndDate
	}
	return p.EndDate
}

func shouldSendStartReminder(plannedStart string, daysBefore int, stats db.ProjectSubtaskStats, today time.Time) bool {
	if plannedStart == "" {
		return false
	}
	if stats.SubtaskAllCompleted || stats.SubtaskAnyActualStart {
		return false
	}
	return db.ShouldRemindInWindow(plannedStart, daysBefore, today)
}

func shouldSendEndReminder(plannedEnd string, daysBefore int, stats db.ProjectSubtaskStats, today time.Time) bool {
	if plannedEnd == "" {
		return false
	}
	if stats.SubtaskAllCompleted {
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
	already, err := db.WasReminderSent(sqlDB, project.ID, kind, sentDate)
	if err != nil {
		return false, err
	}
	if already {
		return false, nil
	}

	extra := wecom.FormatScheduledReminderHeader(kind, daysRemaining(eventDate, today), eventDate)
	msgID, _, _, err := wecom.NotifyProjectMembersEx(project, members, stats, extra)
	if err != nil {
		return false, err
	}

	if err := db.RecordReminderSent(sqlDB, project.ID, kind, sentDate); err != nil {
		slog.Warn("record reminder sent failed", "project_id", project.ID, "kind", kind, "err", err)
	}

	slog.Info("reminder sent",
		"project_id", project.ID,
		"project", project.Name,
		"kind", kind,
		"msgid", msgID,
		"days_remaining", daysRemaining(eventDate, today),
		"event_date", eventDate,
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
