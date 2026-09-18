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
// 统计规则（有计划完成日才统计）：
//   - Total：该部门名下有计划完成日的子任务总数
//   - NotDue：未到期且未完结（不进准时率分母）
//   - OnTime：已完结且实际完成日 ≤ 计划完成日
//   - Rate：OnTime / (Total - NotDue)；分母含「已到期未完结」与「已完结」
//
// 部门归属：取子任务成员所属部门的并集；一人多部门时该子任务在各部门各计 1 次。
type DashboardDeptPunctuality struct {
	DepartmentID   int64   `json:"department_id"`
	DepartmentName string  `json:"department_name"`
	Total          int     `json:"total"`
	NotDue         int     `json:"not_due"`
	OnTime         int     `json:"on_time"`
	Rate           float64 `json:"rate"`
}

type userDeptRef struct {
	ID   int64
	Name string
}

type punctualityClass int

const (
	punctualitySkip punctualityClass = iota
	punctualityNotDue
	punctualityOnTime
	punctualityLate
)

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

// classifySubtaskPunctuality 分类子任务准时状态。
func classifySubtaskPunctuality(s ProjectSubtask) punctualityClass {
	planned, ok := parseDateOnly(s.PlannedEndDate)
	if !ok {
		return punctualitySkip
	}

	if normalizeDateString(s.ActualEndDate) != "" {
		actual, ok := parseDateOnly(s.ActualEndDate)
		if !ok {
			return punctualityLate
		}
		if actual.After(planned) {
			return punctualityLate
		}
		return punctualityOnTime
	}

	if todayDateOnly().After(planned) {
		return punctualityLate
	}
	return punctualityNotDue
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
		notDue int
		onTime int
	}
	buckets := make(map[int64]*bucket)

	add := func(deptID int64, deptName string, class punctualityClass) {
		if class == punctualitySkip {
			return
		}
		b := buckets[deptID]
		if b == nil {
			b = &bucket{name: deptName}
			buckets[deptID] = b
		}
		b.total++
		switch class {
		case punctualityNotDue:
			b.notDue++
		case punctualityOnTime:
			b.onTime++
		}
	}

	for _, project := range projects {
		for _, subtask := range subtasksByProject[project.ID] {
			class := classifySubtaskPunctuality(subtask)
			if class == punctualitySkip {
				continue
			}
			depts := collectSubtaskDepartmentIDs(subtask, userDepts)
			if len(depts) == 0 {
				add(0, dashboardDeptUnassigned, class)
				continue
			}
			for id, name := range depts {
				add(id, name, class)
			}
		}
	}

	out := make([]DashboardDeptPunctuality, 0, len(buckets))
	for id, b := range buckets {
		scored := b.total - b.notDue
		rate := 0.0
		if scored > 0 {
			rate = math.Round(float64(b.onTime)*10000/float64(scored)) / 100
		}
		out = append(out, DashboardDeptPunctuality{
			DepartmentID:   id,
			DepartmentName: b.name,
			Total:          b.total,
			NotDue:         b.notDue,
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
