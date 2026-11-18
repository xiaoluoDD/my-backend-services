package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// DeptProgressCell 某项目下某个参与部门的进度。
// Progress 为 nil 表示尚未填写。
type DeptProgressCell struct {
	DepartmentID   int64  `json:"department_id"`
	DepartmentName string `json:"department_name"`
	Progress       *int   `json:"progress"`
	UpdatedAt      string `json:"updated_at,omitempty"`
	UpdatedBy      string `json:"updated_by,omitempty"`
}

// ProjectDepartmentProgressRow 部门进度表一行（一个项目）。
type ProjectDepartmentProgressRow struct {
	ProjectID   int64              `json:"project_id"`
	Year        string             `json:"year"`
	WorkNo      string             `json:"work_no"`
	Name        string             `json:"name"`
	Departments []DeptProgressCell `json:"departments"`
}

type storedDeptProgress struct {
	Progress  int
	UpdatedAt string
	UpdatedBy string
}

// DepartmentProgressResult 部门进度列表及可选年度。
type DepartmentProgressResult struct {
	Projects []ProjectDepartmentProgressRow `json:"projects"`
	Years    []string                       `json:"years"`
}

// ListDepartmentProgress 按年度返回项目及参与部门进度。
// 参与部门 = 项目负责人 + 项目成员（含子任务成员）的主部门去重。
func ListDepartmentProgress(sqlDB *sql.DB, year string) (*DepartmentProgressResult, error) {
	projects, err := ListProjects(sqlDB)
	if err != nil {
		return nil, err
	}
	years := collectProjectYears(projects)
	projects = filterProjectsByYear(projects, year)

	userMainDept, err := mapUserMainDepartments(sqlDB)
	if err != nil {
		return nil, err
	}

	stored, err := loadAllDepartmentProgress(sqlDB)
	if err != nil {
		return nil, err
	}

	out := make([]ProjectDepartmentProgressRow, 0, len(projects))
	for _, p := range projects {
		members, err := ListProjectMembers(sqlDB, p.ID)
		if err != nil {
			return nil, err
		}
		people := ProjectRecipients(p, members)
		deptMap := make(map[int64]string)
		for _, person := range people {
			ref, ok := userMainDept[strings.TrimSpace(person.UserID)]
			if !ok || ref.ID <= 0 {
				continue
			}
			deptMap[ref.ID] = ref.Name
		}

		cells := make([]DeptProgressCell, 0, len(deptMap))
		for id, name := range deptMap {
			cell := DeptProgressCell{
				DepartmentID:   id,
				DepartmentName: name,
			}
			if s, ok := stored[progressKey{ProjectID: p.ID, DepartmentID: id}]; ok {
				prog := s.Progress
				cell.Progress = &prog
				cell.UpdatedAt = s.UpdatedAt
				cell.UpdatedBy = s.UpdatedBy
			}
			cells = append(cells, cell)
		}
		sort.Slice(cells, func(i, j int) bool {
			li := strings.ToLower(cells[i].DepartmentName)
			lj := strings.ToLower(cells[j].DepartmentName)
			if li != lj {
				return li < lj
			}
			return cells[i].DepartmentID < cells[j].DepartmentID
		})

		out = append(out, ProjectDepartmentProgressRow{
			ProjectID:   p.ID,
			Year:        p.Year,
			WorkNo:      p.WorkNo,
			Name:        p.Name,
			Departments: cells,
		})
	}
	return &DepartmentProgressResult{Projects: out, Years: years}, nil
}

type progressKey struct {
	ProjectID    int64
	DepartmentID int64
}

func loadAllDepartmentProgress(sqlDB *sql.DB) (map[progressKey]storedDeptProgress, error) {
	rows, err := sqlDB.Query(
		`SELECT project_id, department_id, progress, updated_at, updated_by
		 FROM project_department_progress`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[progressKey]storedDeptProgress)
	for rows.Next() {
		var key progressKey
		var s storedDeptProgress
		if err := rows.Scan(&key.ProjectID, &key.DepartmentID, &s.Progress, &s.UpdatedAt, &s.UpdatedBy); err != nil {
			return nil, err
		}
		out[key] = s
	}
	return out, rows.Err()
}

// mapUserMainDepartments 读取成员主部门：优先 app_users.department_id，
// 若为空则取 app_user_departments 中按名称排序的第一个。
func mapUserMainDepartments(sqlDB *sql.DB) (map[string]userDeptRef, error) {
	out := make(map[string]userDeptRef)

	rows, err := sqlDB.Query(`
		SELECT u.userid, u.department_id, COALESCE(d.name, '')
		FROM app_users u
		LEFT JOIN departments d ON d.id = u.department_id
		WHERE u.active=1 AND u.department_id > 0`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var userid, name string
		var id int64
		if err := rows.Scan(&userid, &id, &name); err != nil {
			rows.Close()
			return nil, err
		}
		userid = strings.TrimSpace(userid)
		name = strings.TrimSpace(name)
		if userid == "" || id <= 0 {
			continue
		}
		if name == "" {
			name = "（未命名部门）"
		}
		out[userid] = userDeptRef{ID: id, Name: name}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// 没有 department_id 的成员：从多对多表补一个
	fallback, err := sqlDB.Query(`
		SELECT ud.userid, d.id, d.name
		FROM app_user_departments ud
		INNER JOIN departments d ON d.id = ud.department_id
		INNER JOIN app_users u ON u.userid = ud.userid AND u.active=1
		ORDER BY ud.userid, d.name COLLATE NOCASE, d.id`)
	if err != nil {
		return nil, err
	}
	defer fallback.Close()
	for fallback.Next() {
		var userid, name string
		var id int64
		if err := fallback.Scan(&userid, &id, &name); err != nil {
			return nil, err
		}
		userid = strings.TrimSpace(userid)
		if userid == "" || id <= 0 {
			continue
		}
		if _, ok := out[userid]; ok {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = "（未命名部门）"
		}
		out[userid] = userDeptRef{ID: id, Name: name}
	}
	return out, fallback.Err()
}

// UpsertDepartmentProgress 写入或更新某项目某部门进度（0–100）。
func UpsertDepartmentProgress(sqlDB *sql.DB, projectID, departmentID int64, progress int, updatedBy string) error {
	if projectID <= 0 {
		return fmt.Errorf("无效的项目")
	}
	if departmentID <= 0 {
		return fmt.Errorf("无效的部门")
	}
	if progress < 0 || progress > 100 {
		return fmt.Errorf("进度须为 0–100 的整数")
	}
	if _, err := GetProject(sqlDB, projectID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("项目不存在")
		}
		return err
	}
	var deptName string
	if err := sqlDB.QueryRow(`SELECT name FROM departments WHERE id=?`, departmentID).Scan(&deptName); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("部门不存在")
		}
		return err
	}

	now := time.Now().Format(time.RFC3339)
	_, err := sqlDB.Exec(
		`INSERT INTO project_department_progress (project_id, department_id, progress, updated_at, updated_by)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(project_id, department_id) DO UPDATE SET
		   progress=excluded.progress,
		   updated_at=excluded.updated_at,
		   updated_by=excluded.updated_by`,
		projectID, departmentID, progress, now, strings.TrimSpace(updatedBy),
	)
	return err
}
