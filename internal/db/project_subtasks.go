package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ProjectSubtaskStats 子任务汇总信息（列表展示用）。
type ProjectSubtaskStats struct {
	TaskSummary      string `json:"task_summary"`
	SubtaskStartDate string `json:"subtask_start_date"`
	SubtaskEndDate   string `json:"subtask_end_date"`
}

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

// SummarizeAllProjectSubtaskStats 按项目汇总子任务内容与日期的计划区间。
func SummarizeAllProjectSubtaskStats(db *sql.DB) (map[int64]ProjectSubtaskStats, error) {
	rows, err := db.Query(
		`SELECT project_id, content, planned_start_date, planned_end_date
		 FROM project_subtasks
		 ORDER BY project_id, id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64]ProjectSubtaskStats)
	for rows.Next() {
		var projectID int64
		var content, plannedStart, plannedEnd string
		if err := rows.Scan(&projectID, &content, &plannedStart, &plannedEnd); err != nil {
			return nil, err
		}

		stats := out[projectID]
		content = strings.TrimSpace(content)
		if content != "" {
			if stats.TaskSummary == "" {
				stats.TaskSummary = content
			} else {
				stats.TaskSummary += "；" + content
			}
		}
		if d := normalizeDateString(plannedStart); d != "" {
			if stats.SubtaskStartDate == "" || d < stats.SubtaskStartDate {
				stats.SubtaskStartDate = d
			}
		}
		if d := normalizeDateString(plannedEnd); d != "" {
			if stats.SubtaskEndDate == "" || d > stats.SubtaskEndDate {
				stats.SubtaskEndDate = d
			}
		}
		out[projectID] = stats
	}
	return out, rows.Err()
}

func normalizeDateString(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	layouts := []string{"2006-01-02", "2006-1-2", "2006-1-02", "2006-01-2"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

// SummarizeAllProjectSubtasks 按项目汇总子任务内容（多条以「；」连接）。
func SummarizeAllProjectSubtasks(db *sql.DB) (map[int64]string, error) {
	all, err := SummarizeAllProjectSubtaskStats(db)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]string, len(all))
	for id, stats := range all {
		out[id] = stats.TaskSummary
	}
	return out, nil
}

// SummarizeProjectSubtasks 返回单个项目的子任务内容汇总。
func SummarizeProjectSubtasks(db *sql.DB, projectID int64) (string, error) {
	all, err := SummarizeAllProjectSubtaskStats(db)
	if err != nil {
		return "", err
	}
	return all[projectID].TaskSummary, nil
}

// SummarizeProjectSubtaskStats 返回单个项目的子任务汇总。
func SummarizeProjectSubtaskStats(db *sql.DB, projectID int64) (ProjectSubtaskStats, error) {
	all, err := SummarizeAllProjectSubtaskStats(db)
	if err != nil {
		return ProjectSubtaskStats{}, err
	}
	return all[projectID], nil
}
