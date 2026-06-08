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
