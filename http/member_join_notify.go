package main

import (
	"log/slog"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
	"github.com/xiaoluoDD/my-backend-services/internal/wecom"
)

func addedProjectMembers(before, after []db.ProjectMember) []db.ProjectMember {
	beforeSet := memberUserIDSet(before)
	var added []db.ProjectMember
	for _, m := range after {
		if m.UserID == "" {
			continue
		}
		if _, ok := beforeSet[m.UserID]; !ok {
			added = append(added, m)
		}
	}
	return added
}

func addedSubtaskMembers(before, after []db.ProjectMember) []db.ProjectMember {
	return addedProjectMembers(before, after)
}

func memberUserIDSet(members []db.ProjectMember) map[string]struct{} {
	set := make(map[string]struct{}, len(members))
	for _, m := range members {
		if m.UserID != "" {
			set[m.UserID] = struct{}{}
		}
	}
	return set
}

func shouldSkipJoinNotify(project db.Project, member db.ProjectMember) bool {
	if member.UserID == "" {
		return true
	}
	return member.UserID == project.ManagerUserID
}

func loadProjectJoinContext(projectID int64) (db.Project, db.ProjectSubtaskStats, error) {
	project, err := db.GetProject(sqlDB, projectID)
	if err != nil {
		return db.Project{}, db.ProjectSubtaskStats{}, err
	}
	stats, err := db.SummarizeProjectSubtaskStats(sqlDB, projectID)
	if err != nil {
		return db.Project{}, db.ProjectSubtaskStats{}, err
	}
	return project, stats, nil
}

func notifyNewExplicitProjectMembers(projectID int64, added []db.ProjectMember) {
	if len(added) == 0 {
		slog.Info("member join notify skipped", "project_id", projectID, "reason", "no_new_members")
		return
	}

	project, stats, err := loadProjectJoinContext(projectID)
	if err != nil {
		slog.Warn("member join notify skipped", "project_id", projectID, "err", err)
		return
	}

	sent := 0
	skipped := 0
	for _, member := range added {
		if shouldSkipJoinNotify(project, member) {
			skipped++
			slog.Info("member join notify skip recipient",
				"project_id", projectID,
				"userid", member.UserID,
				"reason", "manager_or_empty",
			)
			continue
		}
		content := wecom.FormatMemberJoinProject(project, stats)
		wecom.NotifyMemberJoinAsync(projectID, member.UserID, content)
		sent++
	}

	slog.Info("member join notify queued",
		"project_id", projectID,
		"added", len(added),
		"sent", sent,
		"skipped", skipped,
	)
}

func notifyNewSubtaskMembers(projectID int64, subtask db.ProjectSubtask, added []db.ProjectMember, wasOnProject map[string]struct{}) {
	if len(added) == 0 {
		return
	}

	project, stats, err := loadProjectJoinContext(projectID)
	if err != nil {
		slog.Warn("subtask join notify skipped", "project_id", projectID, "subtask_id", subtask.ID, "err", err)
		return
	}

	brief := wecom.JoinSubtaskBriefFromModel(subtask)
	for _, member := range added {
		if shouldSkipJoinNotify(project, member) {
			continue
		}
		var content string
		if _, onProject := wasOnProject[member.UserID]; onProject {
			content = wecom.FormatMemberJoinSubtask(project, brief)
		} else {
			content = wecom.FormatMemberJoinProjectAndSubtasks(project, stats, []wecom.JoinSubtaskBrief{brief})
		}
		wecom.NotifyMemberJoinAsync(projectID, member.UserID, content)
	}
}

func projectMemberSnapshot(projectID int64) map[string]struct{} {
	set, err := db.ProjectMemberUserIDSet(sqlDB, projectID)
	if err != nil {
		slog.Warn("project member snapshot failed", "project_id", projectID, "err", err)
		return map[string]struct{}{}
	}
	return set
}
