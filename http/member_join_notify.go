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

func shouldSkipJoinNotify(member db.ProjectMember) bool {
	return member.UserID == ""
}

func notifyProjectMemberJoins(projectID int64, added []db.ProjectMember, ensureManager bool) {
	if len(added) == 0 && !ensureManager {
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
	notified := make(map[string]struct{}, len(added)+1)

	for _, member := range added {
		if shouldSkipJoinNotify(member) {
			skipped++
			continue
		}
		isManager := member.UserID == project.ManagerUserID
		content := wecom.FormatMemberJoinProject(project, stats, isManager)
		wecom.NotifyMemberJoinAsync(projectID, member.UserID, content)
		notified[member.UserID] = struct{}{}
		sent++
	}

	if ensureManager && project.ManagerUserID != "" {
		if _, ok := notified[project.ManagerUserID]; !ok {
			content := wecom.FormatMemberJoinProject(project, stats, true)
			wecom.NotifyMemberJoinAsync(projectID, project.ManagerUserID, content)
			sent++
		}
	}

	slog.Info("member join notify queued",
		"project_id", projectID,
		"added", len(added),
		"sent", sent,
		"skipped", skipped,
		"ensure_manager", ensureManager,
	)
}

func notifyNewExplicitProjectMembers(projectID int64, added []db.ProjectMember) {
	notifyProjectMemberJoins(projectID, added, false)
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
		if shouldSkipJoinNotify(member) {
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
