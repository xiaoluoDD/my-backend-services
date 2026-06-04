package db

import (
	"database/sql"
	"fmt"
	"time"
)

// ProjectSubtask 项目子任务。
type ProjectSubtask struct {
	ID               int64  `json:"id"`
	ProjectID        int64  `json:"project_id"`
	Content          string `json:"content"`
	OwnerUserID      string `json:"owner_userid"`
	OwnerName        string `json:"owner_name"`
	Status           string `json:"status"`
	PlannedStartDate string `json:"planned_start_date"`
	ActualStartDate  string `json:"actual_start_date"`
	PlannedEndDate   string `json:"planned_end_date"`
	ActualEndDate    string `json:"actual_end_date"`
	Remark           string `json:"remark"`
	UpdatedAt        string `json:"updated_at"`
}

// ListProjectSubtasks 返回指定项目的子任务列表。
func ListProjectSubtasks(db *sql.DB, projectID int64) ([]ProjectSubtask, error) {
	if projectID <= 0 {
		return nil, fmt.Errorf("无效的项目 ID")
	}
	rows, err := db.Query(
		`SELECT id, project_id, content, owner_userid, owner_name, status,
		        planned_start_date, actual_start_date, planned_end_date, actual_end_date,
		        remark, updated_at
		 FROM project_subtasks
		 WHERE project_id=?
		 ORDER BY id`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProjectSubtasks(rows)
}

func scanProjectSubtasks(rows *sql.Rows) ([]ProjectSubtask, error) {
	var list []ProjectSubtask
	for rows.Next() {
		var s ProjectSubtask
		if err := rows.Scan(
			&s.ID, &s.ProjectID, &s.Content, &s.OwnerUserID, &s.OwnerName, &s.Status,
			&s.PlannedStartDate, &s.ActualStartDate, &s.PlannedEndDate, &s.ActualEndDate,
			&s.Remark, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

// GetProjectSubtask 按 ID 查询子任务。
func GetProjectSubtask(db *sql.DB, id int64) (ProjectSubtask, error) {
	var s ProjectSubtask
	err := db.QueryRow(
		`SELECT id, project_id, content, owner_userid, owner_name, status,
		        planned_start_date, actual_start_date, planned_end_date, actual_end_date,
		        remark, updated_at
		 FROM project_subtasks WHERE id=?`, id,
	).Scan(
		&s.ID, &s.ProjectID, &s.Content, &s.OwnerUserID, &s.OwnerName, &s.Status,
		&s.PlannedStartDate, &s.ActualStartDate, &s.PlannedEndDate, &s.ActualEndDate,
		&s.Remark, &s.UpdatedAt,
	)
	return s, err
}

// CreateProjectSubtask 新建子任务。
func CreateProjectSubtask(db *sql.DB, s ProjectSubtask) (int64, error) {
	if s.ProjectID <= 0 {
		return 0, fmt.Errorf("无效的项目 ID")
	}
	if s.Content == "" {
		return 0, fmt.Errorf("任务内容不能为空")
	}
	now := time.Now().Format(time.RFC3339)
	res, err := db.Exec(
		`INSERT INTO project_subtasks (
			project_id, content, owner_userid, owner_name, status,
			planned_start_date, actual_start_date, planned_end_date, actual_end_date,
			remark, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ProjectID, s.Content, s.OwnerUserID, s.OwnerName, s.Status,
		s.PlannedStartDate, s.ActualStartDate, s.PlannedEndDate, s.ActualEndDate,
		s.Remark, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateProjectSubtask 更新子任务。
func UpdateProjectSubtask(db *sql.DB, s ProjectSubtask) error {
	if s.ID <= 0 {
		return fmt.Errorf("无效的子任务 ID")
	}
	if s.Content == "" {
		return fmt.Errorf("任务内容不能为空")
	}
	now := time.Now().Format(time.RFC3339)
	res, err := db.Exec(
		`UPDATE project_subtasks SET
			content=?, owner_userid=?, owner_name=?, status=?,
			planned_start_date=?, actual_start_date=?, planned_end_date=?, actual_end_date=?,
			remark=?, updated_at=?
		 WHERE id=? AND project_id=?`,
		s.Content, s.OwnerUserID, s.OwnerName, s.Status,
		s.PlannedStartDate, s.ActualStartDate, s.PlannedEndDate, s.ActualEndDate,
		s.Remark, now, s.ID, s.ProjectID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteProjectSubtask 删除子任务。
func DeleteProjectSubtask(db *sql.DB, id int64) error {
	if id <= 0 {
		return fmt.Errorf("无效的子任务 ID")
	}
	res, err := db.Exec(`DELETE FROM project_subtasks WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
