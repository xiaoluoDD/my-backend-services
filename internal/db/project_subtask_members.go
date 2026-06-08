package db

import (
	"database/sql"
	"fmt"
)

// ListSubtaskMembers 返回子任务成员列表。
func ListSubtaskMembers(db *sql.DB, subtaskID int64) ([]ProjectMember, error) {
	if subtaskID <= 0 {
		return nil, fmt.Errorf("无效的子任务 ID")
	}
	rows, err := db.Query(
		`SELECT sm.userid, sm.name, COALESCE(d.name, u.departments, '')
		 FROM project_subtask_members sm
		 LEFT JOIN app_users u ON sm.userid = u.userid
		 LEFT JOIN departments d ON u.department_id = d.id
		 WHERE sm.subtask_id=?
		 ORDER BY sm.name, sm.userid`,
		subtaskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProjectMembers(rows)
}

// ListSubtaskMembersMapByProject 按项目批量返回子任务成员。
func ListSubtaskMembersMapByProject(db *sql.DB, projectID int64) (map[int64][]ProjectMember, error) {
	if projectID <= 0 {
		return nil, fmt.Errorf("无效的项目 ID")
	}
	rows, err := db.Query(
		`SELECT sm.subtask_id, sm.userid, sm.name, COALESCE(d.name, u.departments, '')
		 FROM project_subtask_members sm
		 INNER JOIN project_subtasks st ON st.id = sm.subtask_id
		 LEFT JOIN app_users u ON sm.userid = u.userid
		 LEFT JOIN departments d ON u.department_id = d.id
		 WHERE st.project_id=?
		 ORDER BY sm.subtask_id, sm.name, sm.userid`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64][]ProjectMember)
	for rows.Next() {
		var subtaskID int64
		var m ProjectMember
		if err := rows.Scan(&subtaskID, &m.UserID, &m.Name, &m.DepartmentName); err != nil {
			return nil, err
		}
		out[subtaskID] = append(out[subtaskID], m)
	}
	return out, rows.Err()
}

// ListSubtaskMembersUnionByProject 返回项目下所有子任务成员（去重）。
func ListSubtaskMembersUnionByProject(db *sql.DB, projectID int64) ([]ProjectMember, error) {
	if projectID <= 0 {
		return nil, fmt.Errorf("无效的项目 ID")
	}
	rows, err := db.Query(
		`SELECT sm.userid, MAX(sm.name), COALESCE(MAX(d.name), MAX(u.departments), '')
		 FROM project_subtask_members sm
		 INNER JOIN project_subtasks st ON st.id = sm.subtask_id
		 LEFT JOIN app_users u ON sm.userid = u.userid
		 LEFT JOIN departments d ON u.department_id = d.id
		 WHERE st.project_id=?
		 GROUP BY sm.userid
		 ORDER BY MAX(sm.name), sm.userid`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProjectMembers(rows)
}

// ListSubtaskMembersUnionMapAllProjects 批量返回各项目的子任务成员（去重）。
func ListSubtaskMembersUnionMapAllProjects(db *sql.DB) (map[int64][]ProjectMember, error) {
	rows, err := db.Query(
		`SELECT st.project_id, sm.userid, MAX(sm.name), COALESCE(MAX(d.name), MAX(u.departments), '')
		 FROM project_subtask_members sm
		 INNER JOIN project_subtasks st ON st.id = sm.subtask_id
		 LEFT JOIN app_users u ON sm.userid = u.userid
		 LEFT JOIN departments d ON u.department_id = d.id
		 GROUP BY st.project_id, sm.userid
		 ORDER BY st.project_id, MAX(sm.name), sm.userid`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64][]ProjectMember)
	for rows.Next() {
		var projectID int64
		var m ProjectMember
		if err := rows.Scan(&projectID, &m.UserID, &m.Name, &m.DepartmentName); err != nil {
			return nil, err
		}
		out[projectID] = append(out[projectID], m)
	}
	return out, rows.Err()
}

func scanProjectMembers(rows *sql.Rows) ([]ProjectMember, error) {
	var list []ProjectMember
	for rows.Next() {
		var m ProjectMember
		if err := rows.Scan(&m.UserID, &m.Name, &m.DepartmentName); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// ReplaceSubtaskMembers 全量替换子任务成员。
func ReplaceSubtaskMembers(db *sql.DB, subtaskID int64, members []ProjectMember) error {
	if subtaskID <= 0 {
		return fmt.Errorf("无效的子任务 ID")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM project_subtask_members WHERE subtask_id=?`, subtaskID); err != nil {
		return err
	}
	for _, m := range members {
		if m.UserID == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO project_subtask_members (subtask_id, userid, name) VALUES (?, ?, ?)`,
			subtaskID, m.UserID, m.Name,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SubtaskRecipients 子任务提醒接收人（负责人 + 子任务成员，去重）。
func SubtaskRecipients(s ProjectSubtask) []ProjectMember {
	seen := make(map[string]struct{})
	out := make([]ProjectMember, 0, len(s.Members)+1)

	add := func(userid, name string) {
		if userid == "" {
			return
		}
		if _, ok := seen[userid]; ok {
			return
		}
		seen[userid] = struct{}{}
		out = append(out, ProjectMember{UserID: userid, Name: name})
	}

	add(s.OwnerUserID, s.OwnerName)
	for _, m := range s.Members {
		add(m.UserID, m.Name)
	}
	return out
}
