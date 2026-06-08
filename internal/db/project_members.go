package db

import (
	"database/sql"
	"fmt"
)

const (
	ProjectMemberSourceExplicit = "explicit"
	ProjectMemberSourceSubtask  = "subtask"
)

// ProjectMember 项目关联的一名成员。
type ProjectMember struct {
	UserID         string `json:"userid"`
	Name           string `json:"name"`
	DepartmentName string `json:"department_name,omitempty"`
}

// ListProjectMembers 返回项目成员（项目编辑成员 + 全部子任务成员，去重展示）。
func ListProjectMembers(db *sql.DB, projectID int64) ([]ProjectMember, error) {
	explicit, err := listProjectMembersBySource(db, projectID, ProjectMemberSourceExplicit)
	if err != nil {
		return nil, err
	}
	subtaskMembers, err := ListSubtaskMembersUnionByProject(db, projectID)
	if err != nil {
		return nil, err
	}
	return MergeMemberLists(explicit, subtaskMembers), nil
}

func listProjectMembersBySource(db *sql.DB, projectID int64, source string) ([]ProjectMember, error) {
	rows, err := db.Query(
		`SELECT pm.userid, pm.name, COALESCE(d.name, u.departments, '')
		 FROM project_members pm
		 LEFT JOIN app_users u ON pm.userid = u.userid
		 LEFT JOIN departments d ON u.department_id = d.id
		 WHERE pm.project_id=? AND pm.source=?
		 ORDER BY pm.name, pm.userid`,
		projectID, source,
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

// ReplaceProjectMembers 全量替换项目编辑时指定的成员（不含负责人与子任务同步成员）。
func ReplaceProjectMembers(db *sql.DB, projectID int64, members []ProjectMember) error {
	if projectID <= 0 {
		return fmt.Errorf("无效的项目 ID")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`DELETE FROM project_members WHERE project_id=? AND source=?`,
		projectID, ProjectMemberSourceExplicit,
	); err != nil {
		return err
	}
	for _, m := range members {
		if m.UserID == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO project_members (project_id, userid, name, source) VALUES (?, ?, ?, ?)
			 ON CONFLICT(project_id, userid) DO UPDATE SET
			   name=excluded.name,
			   source=excluded.source`,
			projectID, m.UserID, m.Name, ProjectMemberSourceExplicit,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SyncProjectMembersFromSubtasks 按当前子任务成员同步 project_members 中的 subtask 来源记录。
func SyncProjectMembersFromSubtasks(db *sql.DB, projectID int64) error {
	if projectID <= 0 {
		return fmt.Errorf("无效的项目 ID")
	}
	union, err := ListSubtaskMembersUnionByProject(db, projectID)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`DELETE FROM project_members WHERE project_id=? AND source=?`,
		projectID, ProjectMemberSourceSubtask,
	); err != nil {
		return err
	}

	for _, m := range union {
		if m.UserID == "" {
			continue
		}
		// 已在项目编辑中指定的成员保留 explicit 记录，不重复插入。
		res, err := tx.Exec(
			`INSERT INTO project_members (project_id, userid, name, source)
			 SELECT ?, ?, ?, ?
			 WHERE NOT EXISTS (
			   SELECT 1 FROM project_members
			   WHERE project_id=? AND userid=? AND source=?
			 )`,
			projectID, m.UserID, m.Name, ProjectMemberSourceSubtask,
			projectID, m.UserID, ProjectMemberSourceExplicit,
		)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			continue
		}
	}

	return tx.Commit()
}

// MergeMemberLists 合并成员列表（按 userid 去重，base 优先保留部门等信息）。
func MergeMemberLists(base, extra []ProjectMember) []ProjectMember {
	seen := make(map[string]int, len(base))
	out := make([]ProjectMember, 0, len(base)+len(extra))
	for _, m := range base {
		if m.UserID == "" {
			continue
		}
		if _, ok := seen[m.UserID]; ok {
			continue
		}
		seen[m.UserID] = len(out)
		out = append(out, m)
	}
	for _, m := range extra {
		if m.UserID == "" {
			continue
		}
		if _, ok := seen[m.UserID]; ok {
			continue
		}
		seen[m.UserID] = len(out)
		out = append(out, m)
	}
	return out
}

// PruneProjectMembersAfterSubtaskRemoval 子任务去掉成员后，若该成员已不在任何子任务中则从 project_members 移除 subtask 来源记录。
func PruneProjectMembersAfterSubtaskRemoval(db *sql.DB, projectID int64, removed []ProjectMember) error {
	if projectID <= 0 || len(removed) == 0 {
		return nil
	}
	union, err := ListSubtaskMembersUnionByProject(db, projectID)
	if err != nil {
		return err
	}
	stillPresent := make(map[string]struct{}, len(union))
	for _, m := range union {
		if m.UserID != "" {
			stillPresent[m.UserID] = struct{}{}
		}
	}
	for _, m := range removed {
		if m.UserID == "" {
			continue
		}
		if _, ok := stillPresent[m.UserID]; ok {
			continue
		}
		if _, err := db.Exec(
			`DELETE FROM project_members WHERE project_id=? AND userid=? AND source=?`,
			projectID, m.UserID, ProjectMemberSourceSubtask,
		); err != nil {
			return err
		}
		// 兼容旧版仅追加写入、source 仍为 explicit 的子任务同步数据。
		if _, err := db.Exec(
			`DELETE FROM project_members WHERE project_id=? AND userid=? AND source=?`,
			projectID, m.UserID, ProjectMemberSourceExplicit,
		); err != nil {
			return err
		}
	}
	return nil
}

// ProjectManagerRecipient 项目负责人（定时/手动项目提醒接收人）。
func ProjectManagerRecipient(p Project) ([]ProjectMember, error) {
	if p.ManagerUserID == "" {
		return nil, fmt.Errorf("该项目未配置负责人，请先编辑项目")
	}
	return []ProjectMember{{UserID: p.ManagerUserID, Name: p.ManagerName}}, nil
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
