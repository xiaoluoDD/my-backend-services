package db

import (
	"database/sql"
	"sort"
	"strings"
)

// DashboardPersonTaskRow 责任人下钻明细行。
type DashboardPersonTaskRow struct {
	ProjectID   int64  `json:"project_id"`
	WorkNo      string `json:"work_no"`
	ProjectName string `json:"project_name"`
	SubtaskID   int64  `json:"subtask_id"`
	Content     string `json:"content"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}

func personKeyMatches(target personKey, userid, name string) bool {
	userid = strings.TrimSpace(userid)
	name = strings.TrimSpace(name)
	if userid != "" && target.userid != "" && userid == target.userid {
		return true
	}
	if name != "" && target.name != "" && name == target.name {
		return true
	}
	return false
}

func normalizeDashboardPersonRole(role string) string {
	role = strings.TrimSpace(role)
	// 兼容旧链接：曾误用 subtask_owner，现统一为子任务成员。
	if role == dashboardPersonRoleSubOwner {
		return dashboardPersonRoleSubMember
	}
	return role
}

func subtaskHasMember(subtask ProjectSubtask, target personKey) bool {
	for _, member := range subtask.Members {
		if personKeyMatches(target, member.UserID, member.Name) {
			return true
		}
	}
	return false
}

// ListDashboardPersonTasks 返回责任人相关的子任务明细。
// status / role 为空表示不过滤；role 取值 project_manager（相关责任人）或 subtask_member（子任务成员）。
func ListDashboardPersonTasks(db *sql.DB, userid, name, status, year, role string) ([]DashboardPersonTaskRow, error) {
	projects, err := ListProjects(db)
	if err != nil {
		return nil, err
	}
	subtasks, err := ListAllProjectSubtasksWithMembers(db)
	if err != nil {
		return nil, err
	}
	subtasksByProject := groupSubtasksByProject(subtasks)

	target := makePersonKey(userid, name)
	if personKeyIsEmpty(target) {
		return []DashboardPersonTaskRow{}, nil
	}

	status = strings.TrimSpace(status)
	role = normalizeDashboardPersonRole(role)
	filtered := filterProjectsByYear(projects, year)
	rowsBySubtaskID := make(map[int64]DashboardPersonTaskRow)

	for _, project := range filtered {
		managerKey := makePersonKey(project.ManagerUserID, project.ManagerName)
		isManager := personKeyMatches(target, managerKey.userid, managerKey.name)

		for _, subtask := range subtasksByProject[project.ID] {
			subtaskStatus := EffectiveSubtaskStatus(subtask)
			if status != "" && subtaskStatus != status {
				continue
			}

			isMember := subtaskHasMember(subtask, target)

			switch role {
			case dashboardPersonRoleManager:
				if !isManager {
					continue
				}
			case dashboardPersonRoleSubMember:
				if !isMember {
					continue
				}
			default:
				if !isManager && !isMember {
					continue
				}
			}

			rowRole := dashboardPersonRoleSubMember
			if role == dashboardPersonRoleManager {
				rowRole = dashboardPersonRoleManager
			} else if role == dashboardPersonRoleSubMember {
				rowRole = dashboardPersonRoleSubMember
			} else if isManager && !isMember {
				rowRole = dashboardPersonRoleManager
			}

			row := DashboardPersonTaskRow{
				ProjectID:   project.ID,
				WorkNo:      strings.TrimSpace(project.WorkNo),
				ProjectName: strings.TrimSpace(project.Name),
				SubtaskID:   subtask.ID,
				Content:     strings.TrimSpace(subtask.Content),
				Role:        rowRole,
				Status:      subtaskStatus,
			}
			if existing, ok := rowsBySubtaskID[subtask.ID]; ok {
				if existing.Role == dashboardPersonRoleManager && rowRole == dashboardPersonRoleSubMember {
					rowsBySubtaskID[subtask.ID] = row
				}
				continue
			}
			rowsBySubtaskID[subtask.ID] = row
		}
	}

	rows := make([]DashboardPersonTaskRow, 0, len(rowsBySubtaskID))
	for _, row := range rowsBySubtaskID {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		left := strings.ToLower(rows[i].WorkNo + rows[i].ProjectName + rows[i].Content)
		right := strings.ToLower(rows[j].WorkNo + rows[j].ProjectName + rows[j].Content)
		return left < right
	})
	return rows, nil
}
