package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Project 项目看板一行记录。
type Project struct {
	ID            int64  `json:"id"`
	Year          string `json:"year"`
	WorkNo        string `json:"work_no"`
	Name          string `json:"name"`
	ManagerUserID string `json:"manager_userid"`
	ManagerName   string `json:"manager_name"`
	GroupChat     string `json:"group_chat"`
	GroupChatID   string `json:"group_chat_id"`
	Status        string `json:"status"`
	StartDate     string `json:"start_date"`
	EndDate       string `json:"end_date"`
	Tasks         string `json:"tasks"`
	UpdatedAt     string `json:"updated_at"`
}

// ListProjects 返回全部项目（按年度、工番号排序）。
func ListProjects(db *sql.DB) ([]Project, error) {
	rows, err := db.Query(
		`SELECT id, year, work_no, name, manager_userid, manager_name,
		        group_chat, group_chat_id, status, start_date, end_date, tasks, updated_at
		 FROM projects ORDER BY year DESC, work_no, id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(
			&p.ID, &p.Year, &p.WorkNo, &p.Name, &p.ManagerUserID, &p.ManagerName,
			&p.GroupChat, &p.GroupChatID, &p.Status, &p.StartDate, &p.EndDate, &p.Tasks, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// GetProject 按 ID 查询项目。
func GetProject(db *sql.DB, id int64) (Project, error) {
	var p Project
	err := db.QueryRow(
		`SELECT id, year, work_no, name, manager_userid, manager_name,
		        group_chat, group_chat_id, status, start_date, end_date, tasks, updated_at
		 FROM projects WHERE id=?`, id,
	).Scan(
		&p.ID, &p.Year, &p.WorkNo, &p.Name, &p.ManagerUserID, &p.ManagerName,
		&p.GroupChat, &p.GroupChatID, &p.Status, &p.StartDate, &p.EndDate, &p.Tasks, &p.UpdatedAt,
	)
	return p, err
}

// CreateProject 新建项目（不创建企微群）。
func CreateProject(db *sql.DB, p Project) (int64, error) {
	now := time.Now().Format(time.RFC3339)
	res, err := db.Exec(
		`INSERT INTO projects (
			year, work_no, name, manager_userid, manager_name,
			group_chat, group_chat_id, status, start_date, end_date, tasks, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Year, p.WorkNo, p.Name, p.ManagerUserID, p.ManagerName,
		p.GroupChat, p.GroupChatID, p.Status, p.StartDate, p.EndDate, p.Tasks, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateProject 更新项目。
func UpdateProject(db *sql.DB, p Project) error {
	if p.ID <= 0 {
		return fmt.Errorf("无效的项目 ID")
	}
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(
		`UPDATE projects SET
			year=?, work_no=?, name=?, manager_userid=?, manager_name=?,
			group_chat=?, group_chat_id=?, status=?, start_date=?, end_date=?, tasks=?, updated_at=?
		 WHERE id=?`,
		p.Year, p.WorkNo, p.Name, p.ManagerUserID, p.ManagerName,
		p.GroupChat, p.GroupChatID, p.Status, p.StartDate, p.EndDate, p.Tasks, now,
		p.ID,
	)
	return err
}

// AllProjectSubtasksCompleted 判断项目子任务是否均可视为已完结。
// 无子任务时返回 allDone=true；有子任务则须全部填写实际完成日期。
func AllProjectSubtasksCompleted(dbConn *sql.DB, projectID int64) (allDone bool, incomplete int, total int, err error) {
	list, err := ListProjectSubtasks(dbConn, projectID)
	if err != nil {
		return false, 0, 0, err
	}
	total = len(list)
	for _, s := range list {
		if normalizeDateString(s.ActualEndDate) == "" {
			incomplete++
		}
	}
	return incomplete == 0, incomplete, total, nil
}

// ClearProjectEndDate 若项目已填写实际完结日期则清空，并按日期规则重算状态。
func ClearProjectEndDate(dbConn *sql.DB, projectID int64) (cleared bool, err error) {
	p, err := GetProject(dbConn, projectID)
	if err != nil {
		return false, err
	}
	if normalizeDateString(p.EndDate) == "" {
		return false, nil
	}
	p.EndDate = ""
	SyncProjectStatus(&p)
	if err := UpdateProject(dbConn, p); err != nil {
		return false, err
	}
	return true, nil
}

// DeleteProject 删除项目。
func DeleteProject(db *sql.DB, id int64) error {
	if id <= 0 {
		return fmt.Errorf("无效的项目 ID")
	}
	_, err := db.Exec(`DELETE FROM projects WHERE id=?`, id)
	return err
}

func seedProjectsIfEmpty(db *sql.DB) error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	now := time.Now().Format(time.RFC3339)
	examples := []Project{
		{
			Year: "2026", WorkNo: "A-001", Name: "示例项目 A",
			ManagerName: "（待填写）", GroupChat: "（待绑定群聊）",
			Status: "进行中", StartDate: "2026-01-01", EndDate: "2026-12-31",
			Tasks: "需求确认；开发；验收", UpdatedAt: now,
		},
		{
			Year: "2026", WorkNo: "B-002", Name: "示例项目 B",
			ManagerName: "（待填写）", Status: "待启动",
			StartDate: "2026-03-01", Tasks: "立项评审", UpdatedAt: now,
		},
	}

	for _, p := range examples {
		if _, err := db.Exec(
			`INSERT INTO projects (
				year, work_no, name, manager_userid, manager_name,
				group_chat, group_chat_id, status, start_date, end_date, tasks, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.Year, p.WorkNo, p.Name, p.ManagerUserID, p.ManagerName,
			p.GroupChat, p.GroupChatID, p.Status, p.StartDate, p.EndDate, p.Tasks, p.UpdatedAt,
		); err != nil {
			return fmt.Errorf("seed project: %w", err)
		}
	}
	return nil
}
