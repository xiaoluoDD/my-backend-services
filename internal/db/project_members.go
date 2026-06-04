package db

import (
	"database/sql"
	"fmt"
)

// ProjectMember 项目关联的一名成员。
type ProjectMember struct {
	UserID         string `json:"userid"`
	Name           string `json:"name"`
	DepartmentName string `json:"department_name,omitempty"`
}

// ListProjectMembers 返回项目成员列表（含部门名称）。
func ListProjectMembers(db *sql.DB, projectID int64) ([]ProjectMember, error) {
	rows, err := db.Query(
		`SELECT pm.userid, pm.name, COALESCE(d.name, u.departments, '')
		 FROM project_members pm
		 LEFT JOIN app_users u ON pm.userid = u.userid
		 LEFT JOIN departments d ON u.department_id = d.id
		 WHERE pm.project_id=?
		 ORDER BY pm.name, pm.userid`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

// ReplaceProjectMembers 全量替换项目成员（不含负责人，负责人见 projects.manager_userid）。
func ReplaceProjectMembers(db *sql.DB, projectID int64, members []ProjectMember) error {
	if projectID <= 0 {
		return fmt.Errorf("无效的项目 ID")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM project_members WHERE project_id=?`, projectID); err != nil {
		return err
	}
	for _, m := range members {
		if m.UserID == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO project_members (project_id, userid, name) VALUES (?, ?, ?)`,
			projectID, m.UserID, m.Name,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ProjectRecipients 项目提醒接收人（负责人 + 项目成员，去重）。
func ProjectRecipients(p Project, members []ProjectMember) []ProjectMember {
	seen := make(map[string]struct{})
	out := make([]ProjectMember, 0, len(members)+1)

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

	add(p.ManagerUserID, p.ManagerName)
	for _, m := range members {
		add(m.UserID, m.Name)
	}
	return out
}
