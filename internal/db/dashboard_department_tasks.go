package db

import (
	"database/sql"
	"sort"
	"strconv"
	"strings"
)

// DashboardDepartmentTaskRow 部门准时率下钻明细。
type DashboardDepartmentTaskRow struct {
	ProjectID      int64  `json:"project_id"`
	WorkNo         string `json:"work_no"`
	ProjectName    string `json:"project_name"`
	SubtaskID      int64  `json:"subtask_id"`
	Content        string `json:"content"`
	Status         string `json:"status"`
	Punctuality    string `json:"punctuality"`
	PlannedEndDate string `json:"planned_end_date"`
	ActualEndDate  string `json:"actual_end_date"`
}

func punctualityLabel(class punctualityClass) string {
	switch class {
	case punctualityOnTime:
		return "准时"
	case punctualityNotDue:
		return "未到期"
	case punctualityLate:
		return "不准时"
	default:
		return ""
	}
}

func punctualityKindMatch(class punctualityClass, kind string) bool {
	if class == punctualitySkip {
		return false
	}
	switch strings.TrimSpace(kind) {
	case "", "all":
		return true
	case "not_due":
		return class == punctualityNotDue
	case "on_time":
		return class == punctualityOnTime
	case "late":
		return class == punctualityLate
	case "scored":
		return class == punctualityOnTime || class == punctualityLate
	default:
		return true
	}
}

func subtaskMatchesDepartment(subtask ProjectSubtask, userDepts map[string][]userDeptRef, departmentID int64, departmentName string) bool {
	depts := collectSubtaskDepartmentIDs(subtask, userDepts)
	if departmentID > 0 {
		_, ok := depts[departmentID]
		return ok
	}
	name := strings.TrimSpace(departmentName)
	if name == "" || name == dashboardDeptUnassigned {
		return len(depts) == 0
	}
	for _, deptName := range depts {
		if deptName == name {
			return true
		}
	}
	return false
}

// ListDashboardDepartmentTasks 返回某个部门准时率统计里的子任务。
// kind 为空表示该部门全部纳入统计的子任务；not_due / on_time / late / scored 为子集。
// departmentID 为 0 时按部门名匹配，空名或「未分配部门」表示没有部门的子任务。
func ListDashboardDepartmentTasks(db *sql.DB, departmentID int64, departmentName, year, kind string) ([]DashboardDepartmentTaskRow, error) {
	projects, err := ListProjects(db)
	if err != nil {
		return nil, err
	}
	subtasks, err := ListAllProjectSubtasksWithMembers(db)
	if err != nil {
		return nil, err
	}
	userDepts, err := mapUserDepartments(db)
	if err != nil {
		return nil, err
	}
	subtasksByProject := groupSubtasksByProject(subtasks)
	filtered := filterProjectsByYear(projects, year)

	rows := make([]DashboardDepartmentTaskRow, 0)
	seen := make(map[int64]struct{})
	for _, project := range filtered {
		for _, subtask := range subtasksByProject[project.ID] {
			class := classifySubtaskPunctuality(subtask)
			if !punctualityKindMatch(class, kind) {
				continue
			}
			if !subtaskMatchesDepartment(subtask, userDepts, departmentID, departmentName) {
				continue
			}
			if _, ok := seen[subtask.ID]; ok {
				continue
			}
			seen[subtask.ID] = struct{}{}
			rows = append(rows, DashboardDepartmentTaskRow{
				ProjectID:      project.ID,
				WorkNo:         strings.TrimSpace(project.WorkNo),
				ProjectName:    strings.TrimSpace(project.Name),
				SubtaskID:      subtask.ID,
				Content:        strings.TrimSpace(subtask.Content),
				Status:         EffectiveSubtaskStatus(subtask),
				Punctuality:    punctualityLabel(class),
				PlannedEndDate: strings.TrimSpace(subtask.PlannedEndDate),
				ActualEndDate:  strings.TrimSpace(subtask.ActualEndDate),
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		left := strings.ToLower(rows[i].WorkNo + rows[i].ProjectName + rows[i].Content)
		right := strings.ToLower(rows[j].WorkNo + rows[j].ProjectName + rows[j].Content)
		if left != right {
			return left < right
		}
		return rows[i].SubtaskID < rows[j].SubtaskID
	})
	return rows, nil
}

// ParseDepartmentID 解析下钻参数。空字符串表示未提供。
func ParseDepartmentID(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 0 {
		return 0, false
	}
	return id, true
}
