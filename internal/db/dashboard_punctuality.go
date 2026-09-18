package db

import (
	"database/sql"
	"math"
	"sort"
	"strings"
)

const dashboardDeptUnassigned = "（未分配部门）"

// DashboardDeptPunctuality 部门子任务准时率。
//
// 统计规则：
//   - 已完结：实际完成日 ≤ 计划完成日 → 准时；否则不准时
//   - 未完结且已过计划完成日 → 不准时
//   - 未到期且未完结 → 不计入分母
//   - 无计划完成日 → 不计入
//
// 部门归属：取子任务成员所属部门的并集；一人多部门时该子任务在各部门各计 1 次。
type DashboardDeptPunctuality struct {
	DepartmentID   int64   `json:"department_id"`
	DepartmentName string  `json:"department_name"`
	Total          int     `json:"total"`
	OnTime         int     `json:"on_time"`
	Rate           float64 `json:"rate"`
}

type userDeptRef struct {
	ID   int64
	Name string
}

func mapUserDepartments(db *sql.DB) (map[string][]userDeptRef, error) {
	rows, err := db.Query(`
		SELECT ud.userid, d.id, d.name
		FROM app_user_departments ud
		INNER JOIN departments d ON d.id = ud.department_id
		ORDER BY ud.userid, d.name COLLATE NOCASE, d.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]userDeptRef)
	for rows.Next() {
		var userid, name string
		var id int64
		if err := rows.Scan(&userid, &id, &name); err != nil {
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
		out[userid] = append(out[userid], userDeptRef{ID: id, Name: name})
	}
	return out, rows.Err()
}

// classifySubtaskPunctuality 返回是否纳入统计、是否准时。
func classifySubtaskPunctuality(s ProjectSubtask) (included bool, onTime bool) {
	planned, ok := parseDateOnly(s.PlannedEndDate)
	if !ok {
		return false, false
	}

	if normalizeDateString(s.ActualEndDate) != "" {
		actual, ok := parseDateOnly(s.ActualEndDate)
		if !ok {
			return true, false
		}
		// 实际完成日 ≤ 计划完成日
		return true, !actual.After(planned)
	}

	// 未完结：已过计划完成日计不准时；未到期不计入
	if todayDateOnly().After(planned) {
		return true, false
	}
	return false, false
}

func collectSubtaskDepartmentIDs(subtask ProjectSubtask, userDepts map[string][]userDeptRef) map[int64]string {
	depts := make(map[int64]string)
	for _, member := range subtask.Members {
		userid := strings.TrimSpace(member.UserID)
		if userid == "" {
			continue
		}
		for _, d := range userDepts[userid] {
			depts[d.ID] = d.Name
		}
	}
	return depts
}

func buildDepartmentPunctuality(
	projects []Project,
	subtasksByProject map[int64][]ProjectSubtask,
	userDepts map[string][]userDeptRef,
) []DashboardDeptPunctuality {
	type bucket struct {
		name   string
		total  int
		onTime int
	}
	buckets := make(map[int64]*bucket)

	add := func(deptID int64, deptName string, onTime bool) {
		b := buckets[deptID]
		if b == nil {
			b = &bucket{name: deptName}
			buckets[deptID] = b
		}
		b.total++
		if onTime {
			b.onTime++
		}
	}

	for _, project := range projects {
		for _, subtask := range subtasksByProject[project.ID] {
			included, onTime := classifySubtaskPunctuality(subtask)
			if !included {
				continue
			}
			depts := collectSubtaskDepartmentIDs(subtask, userDepts)
			if len(depts) == 0 {
				add(0, dashboardDeptUnassigned, onTime)
				continue
			}
			for id, name := range depts {
				add(id, name, onTime)
			}
		}
	}

	out := make([]DashboardDeptPunctuality, 0, len(buckets))
	for id, b := range buckets {
		rate := 0.0
		if b.total > 0 {
			rate = math.Round(float64(b.onTime)*10000/float64(b.total)) / 100
		}
		out = append(out, DashboardDeptPunctuality{
			DepartmentID:   id,
			DepartmentName: b.name,
			Total:          b.total,
			OnTime:         b.onTime,
			Rate:           rate,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Rate != out[j].Rate {
			return out[i].Rate < out[j].Rate
		}
		li := strings.ToLower(out[i].DepartmentName)
		lj := strings.ToLower(out[j].DepartmentName)
		if li != lj {
			return li < lj
		}
		return out[i].DepartmentID < out[j].DepartmentID
	})
	return out
}
