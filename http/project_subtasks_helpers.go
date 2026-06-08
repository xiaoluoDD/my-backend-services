package main

import (
	"github.com/xiaoluoDD/my-backend-services/internal/db"
)

func attachSubtaskMembers(projectID int64, list []db.ProjectSubtask) ([]db.ProjectSubtask, error) {
	if len(list) == 0 {
		return list, nil
	}
	membersMap, err := db.ListSubtaskMembersMapByProject(sqlDB, projectID)
	if err != nil {
		return nil, err
	}
	for i := range list {
		members := membersMap[list[i].ID]
		if members == nil {
			members = []db.ProjectMember{}
		}
		list[i].Members = members
	}
	return list, nil
}

func loadSubtaskWithMembers(id int64) (db.ProjectSubtask, error) {
	s, err := db.GetProjectSubtask(sqlDB, id)
	if err != nil {
		return s, err
	}
	members, err := db.ListSubtaskMembers(sqlDB, id)
	if err != nil {
		return s, err
	}
	if members == nil {
		members = []db.ProjectMember{}
	}
	s.Members = members
	s.Status = db.EffectiveSubtaskStatus(s)
	return s, nil
}

func syncSubtaskMembersToProject(projectID int64) error {
	return db.SyncProjectMembersFromSubtasks(sqlDB, projectID)
}

func syncSubtaskMembersToProjectAfterChange(projectID int64, removed []db.ProjectMember) error {
	if err := db.SyncProjectMembersFromSubtasks(sqlDB, projectID); err != nil {
		return err
	}
	return db.PruneProjectMembersAfterSubtaskRemoval(sqlDB, projectID, removed)
}

func removedSubtaskMembers(before, after []db.ProjectMember) []db.ProjectMember {
	afterSet := make(map[string]struct{}, len(after))
	for _, m := range after {
		if m.UserID != "" {
			afterSet[m.UserID] = struct{}{}
		}
	}
	var removed []db.ProjectMember
	for _, m := range before {
		if m.UserID == "" {
			continue
		}
		if _, ok := afterSet[m.UserID]; !ok {
			removed = append(removed, m)
		}
	}
	return removed
}
