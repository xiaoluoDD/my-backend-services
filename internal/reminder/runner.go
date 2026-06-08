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
	StartSent            int      `json:"start_sent"`
	EndSent              int      `json:"end_sent"`
	SubtaskMgrDigestSent int      `json:"subtask_mgr_digest_sent"`
	SubtaskMemberDigest  int      `json:"subtask_member_digest_sent"`
	Skipped              int      `json:"skipped"`
	Errors               []string `json:"errors,omitempty"`
	RunDate              string   `json:"run_date"`
}

type projectDigestItem struct {
	Project       db.Project
	EventDate     string
	DaysRemaining int
}

type subtaskDigestItem struct {
	Subtask       db.ProjectSubtask
	Project       db.Project
	DaysRemaining int
	EventDate     string
}

// RunDaily 扫描项目与子任务，按设置发送摘要提醒（方案 C）。
func RunDaily(sqlDB *sql.DB, settings db.AppSettings) RunResult {
	result := RunResult{RunDate: time.Now().Format("2006-01-02")}
	today := time.Now()
	startDays := settings.StartReminderDays()
	endDays := settings.EndReminderDays()

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

	projectMap := make(map[int64]db.Project, len(projects))
	for _, p := range projects {
		projectMap[p.ID] = p
	}

	runProjectDigests(sqlDB, projects, statsMap, startDays, endDays, today, &result)

	subtasks, err := db.ListAllProjectSubtasksWithMembers(sqlDB)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
	} else {
		runSubtaskDigests(sqlDB, subtasks, projectMap, startDays, today, &result)
	}

	slog.Info("reminder · finished",
		"date", result.RunDate,
		"projects", len(projects),
		"start", result.StartSent,
		"end", result.EndSent,
		"subtask_mgr", result.SubtaskMgrDigestSent,
		"subtask_member", result.SubtaskMemberDigest,
		"skipped", result.Skipped,
		"errors", len(result.Errors),
	)
	return result
}

func runProjectDigests(
	sqlDB *sql.DB,
	projects []db.Project,
	statsMap map[int64]db.ProjectSubtaskStats,
	startDays, endDays int,
	today time.Time,
	result *RunResult,
) {
	sentDate := today.Format("2006-01-02")
	mgrStart := make(map[string][]projectDigestItem)
	mgrEnd := make(map[string][]projectDigestItem)

	for _, p := range projects {
		if p.ManagerUserID == "" {
			continue
		}
		if shouldSendProjectStart(p, startDays, today) {
			if !projectReminderBlocked(sqlDB, p.ID, db.ReminderKindStart, sentDate) {
				mgrStart[p.ManagerUserID] = append(mgrStart[p.ManagerUserID], projectDigestItem{
					Project:       p,
					EventDate:     p.StartDate,
					DaysRemaining: daysRemaining(p.StartDate, today),
				})
			} else {
				result.Skipped++
			}
		}

		plannedEnd := statsMap[p.ID].SubtaskEndDate
		if shouldSendProjectEnd(p, plannedEnd, endDays, today) {
			if !projectReminderBlocked(sqlDB, p.ID, db.ReminderKindEnd, sentDate) {
				mgrEnd[p.ManagerUserID] = append(mgrEnd[p.ManagerUserID], projectDigestItem{
					Project:       p,
					EventDate:     plannedEnd,
					DaysRemaining: daysRemaining(plannedEnd, today),
				})
			} else {
				result.Skipped++
			}
		}
	}

	for mgrID, items := range mgrStart {
		if len(items) == 0 {
			continue
		}
		lines := projectDigestLines(items)
		content := wecom.FormatProjectStartDigest(lines)
		msgID, err := wecom.NotifyUsers([]string{mgrID}, content)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("负责人 %s 项目启动摘要: %v", mgrID, err))
			continue
		}
		recordProjectDigestSent(sqlDB, items, db.ReminderKindStart, sentDate)
		result.StartSent++
		slog.Info("reminder · project start digest", "mgr", mgrID, "items", len(items), "msgid", msgID)
	}

	for mgrID, items := range mgrEnd {
		if len(items) == 0 {
			continue
		}
		lines := projectDigestLines(items)
		content := wecom.FormatProjectEndDigest(lines)
		msgID, err := wecom.NotifyUsers([]string{mgrID}, content)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("负责人 %s 项目完结摘要: %v", mgrID, err))
			continue
		}
		recordProjectDigestSent(sqlDB, items, db.ReminderKindEnd, sentDate)
		result.EndSent++
		slog.Info("reminder · project end digest", "mgr", mgrID, "items", len(items), "msgid", msgID)
	}
}

func projectDigestLines(items []projectDigestItem) []wecom.ProjectDigestLine {
	lines := make([]wecom.ProjectDigestLine, 0, len(items))
	for _, it := range items {
		mgr := it.Project.ManagerName
		if mgr == "" {
			mgr = it.Project.ManagerUserID
		}
		lines = append(lines, wecom.ProjectDigestLine{
			Name:          it.Project.Name,
			WorkNo:        it.Project.WorkNo,
			ManagerName:   mgr,
			Status:        db.EffectiveProjectStatus(it.Project),
			EventDate:     it.EventDate,
			DaysRemaining: it.DaysRemaining,
		})
	}
	return lines
}

func recordProjectDigestSent(sqlDB *sql.DB, items []projectDigestItem, kind, sentDate string) {
	if !oncePerDayReminderDedup {
		return
	}
	for _, it := range items {
		_ = db.RecordReminderSent(sqlDB, it.Project.ID, kind, sentDate)
	}
}

func projectReminderBlocked(sqlDB *sql.DB, projectID int64, kind, sentDate string) bool {
	if !oncePerDayReminderDedup {
		return false
	}
	already, err := db.WasReminderSent(sqlDB, projectID, kind, sentDate)
	return err == nil && already
}

func shouldSendProjectStart(p db.Project, daysBefore int, today time.Time) bool {
	if p.StartDate == "" {
		return false
	}
	if db.EffectiveProjectStatus(p) != db.ProjectStatusNotStarted {
		return false
	}
	return db.ShouldRemindInWindow(p.StartDate, daysBefore, today)
}

func shouldSendProjectEnd(p db.Project, plannedEnd string, daysBefore int, today time.Time) bool {
	if plannedEnd == "" {
		return false
	}
	if db.ProjectHasActualEnd(p) {
		return false
	}
	if db.EffectiveProjectStatus(p) != db.ProjectStatusInProgress {
		return false
	}
	return db.ShouldRemindInWindow(plannedEnd, daysBefore, today)
}

func runSubtaskDigests(
	sqlDB *sql.DB,
	subtasks []db.ProjectSubtask,
	projectMap map[int64]db.Project,
	startDays int,
	today time.Time,
	result *RunResult,
) {
	sentDate := today.Format("2006-01-02")

	mgrBuckets := make(map[int64][]subtaskDigestItem)
	memberBuckets := make(map[string][]subtaskDigestItem)
	memberNames := make(map[string]string)

	for _, st := range subtasks {
		if !shouldSendSubtaskStart(st, startDays, today) {
			continue
		}
		if subtaskReminderBlocked(sqlDB, st.ID, sentDate) {
			result.Skipped++
			continue
		}

		project, ok := projectMap[st.ProjectID]
		if !ok {
			continue
		}
		if db.EffectiveProjectStatus(project) == db.ProjectStatusCompleted {
			continue
		}

		item := subtaskDigestItem{
			Subtask:       st,
			Project:       project,
			DaysRemaining: daysRemaining(st.PlannedStartDate, today),
			EventDate:     st.PlannedStartDate,
		}
		mgrBuckets[project.ID] = append(mgrBuckets[project.ID], item)

		mgrID := project.ManagerUserID
		for _, m := range st.Members {
			if m.UserID == "" || m.UserID == mgrID {
				continue
			}
			memberBuckets[m.UserID] = append(memberBuckets[m.UserID], item)
			if m.Name != "" {
				memberNames[m.UserID] = m.Name
			}
		}
	}

	for projectID, items := range mgrBuckets {
		if len(items) == 0 {
			continue
		}
		project := items[0].Project
		if project.ManagerUserID == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("项目 %d 无负责人，跳过子任务摘要", projectID))
			continue
		}
		if projectDigestBlocked(sqlDB, projectID, db.ReminderKindSubtaskStartDigestMgr, sentDate) {
			result.Skipped++
			continue
		}

		mgrName := project.ManagerName
		if mgrName == "" {
			mgrName = project.ManagerUserID
		}
		content := wecom.FormatManagerSubtaskDigest(
			project.Name, project.WorkNo, mgrName, subtaskDigestLines(items))
		msgID, err := wecom.NotifyUsers([]string{project.ManagerUserID}, content)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("项目 %d 子任务摘要: %v", projectID, err))
			continue
		}

		recordSubtaskDigestSent(sqlDB, projectID, items, db.ReminderKindSubtaskStartDigestMgr, sentDate)
		result.SubtaskMgrDigestSent++
		slog.Info("reminder · subtask digest mgr",
			"project_id", projectID,
			"items", len(items),
			"msgid", msgID,
		)
	}

	for userID, items := range memberBuckets {
		if len(items) == 0 {
			continue
		}
		if userDigestBlocked(sqlDB, userID, db.ReminderKindSubtaskStartDigestMember, sentDate) {
			result.Skipped++
			continue
		}

		lines, seen := uniqueSubtaskLines(items)
		content := wecom.FormatMemberSubtaskDigest(lines)
		msgID, err := wecom.NotifyUsers([]string{userID}, content)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("成员 %s 子任务摘要: %v", userID, err))
			continue
		}

		recordMemberSubtaskDigestSent(sqlDB, userID, seen, sentDate)
		result.SubtaskMemberDigest++
		slog.Info("reminder · subtask digest member",
			"userid", userID,
			"name", memberNames[userID],
			"items", len(lines),
			"msgid", msgID,
		)
	}
}

func subtaskDigestLines(items []subtaskDigestItem) []wecom.SubtaskDigestLine {
	lines := make([]wecom.SubtaskDigestLine, 0, len(items))
	for _, it := range items {
		mgr := it.Project.ManagerName
		if mgr == "" {
			mgr = it.Project.ManagerUserID
		}
		lines = append(lines, wecom.SubtaskDigestLine{
			ProjectName:   it.Project.Name,
			WorkNo:        it.Project.WorkNo,
			ManagerName:   mgr,
			Content:       it.Subtask.Content,
			EventDate:     it.EventDate,
			DaysRemaining: it.DaysRemaining,
		})
	}
	return lines
}

func uniqueSubtaskLines(items []subtaskDigestItem) ([]wecom.SubtaskDigestLine, map[int64]struct{}) {
	seen := make(map[int64]struct{})
	lines := make([]wecom.SubtaskDigestLine, 0, len(items))
	for _, it := range items {
		if _, ok := seen[it.Subtask.ID]; ok {
			continue
		}
		seen[it.Subtask.ID] = struct{}{}
		mgr := it.Project.ManagerName
		if mgr == "" {
			mgr = it.Project.ManagerUserID
		}
		lines = append(lines, wecom.SubtaskDigestLine{
			ProjectName:   it.Project.Name,
			WorkNo:        it.Project.WorkNo,
			ManagerName:   mgr,
			Content:       it.Subtask.Content,
			EventDate:     it.EventDate,
			DaysRemaining: it.DaysRemaining,
		})
	}
	return lines, seen
}

func subtaskReminderBlocked(sqlDB *sql.DB, subtaskID int64, sentDate string) bool {
	if !oncePerDayReminderDedup {
		return false
	}
	already, err := db.WasSubtaskReminderSent(sqlDB, subtaskID, db.ReminderKindSubtaskStart, sentDate)
	return err == nil && already
}

func projectDigestBlocked(sqlDB *sql.DB, projectID int64, kind, sentDate string) bool {
	if !oncePerDayReminderDedup {
		return false
	}
	already, err := db.WasReminderSent(sqlDB, projectID, kind, sentDate)
	return err == nil && already
}

func userDigestBlocked(sqlDB *sql.DB, userID, kind, sentDate string) bool {
	if !oncePerDayReminderDedup {
		return false
	}
	already, err := db.WasUserReminderSent(sqlDB, userID, kind, sentDate)
	return err == nil && already
}

func recordSubtaskDigestSent(sqlDB *sql.DB, projectID int64, items []subtaskDigestItem, kind, sentDate string) {
	if !oncePerDayReminderDedup {
		return
	}
	_ = db.RecordReminderSent(sqlDB, projectID, kind, sentDate)
	for _, it := range items {
		_ = db.RecordSubtaskReminderSent(sqlDB, it.Subtask.ID, db.ReminderKindSubtaskStart, sentDate)
	}
}

func recordMemberSubtaskDigestSent(sqlDB *sql.DB, userID string, seen map[int64]struct{}, sentDate string) {
	if !oncePerDayReminderDedup {
		return
	}
	_ = db.RecordUserReminderSent(sqlDB, userID, db.ReminderKindSubtaskStartDigestMember, sentDate)
	for id := range seen {
		_ = db.RecordSubtaskReminderSent(sqlDB, id, db.ReminderKindSubtaskStart, sentDate)
	}
}

func shouldSendSubtaskStart(st db.ProjectSubtask, daysBefore int, today time.Time) bool {
	if st.PlannedStartDate == "" {
		return false
	}
	if db.EffectiveSubtaskStatus(st) != db.SubtaskStatusNotStarted {
		return false
	}
	return db.ShouldRemindInWindow(st.PlannedStartDate, daysBefore, today)
}

func daysRemaining(eventDate string, today time.Time) int {
	d, ok := db.DaysUntilEvent(eventDate, today)
	if !ok || d < 0 {
		return 0
	}
	return d
}
