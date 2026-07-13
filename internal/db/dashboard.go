package db

import (
	"database/sql"
	"sort"
	"strings"
)

// DashboardStatusCount 状态计数。
type DashboardStatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// DashboardProjectSummary 项目概览 KPI。
type DashboardProjectSummary struct {
	Total      int `json:"total"`
	NotStarted int `json:"not_started"`
	InProgress int `json:"in_progress"`
	Overdue    int `json:"overdue"`
	Completed  int `json:"completed"`
}

// DashboardWorkNoGroup 按工番号分组。
type DashboardWorkNoGroup struct {
	WorkNo    string                 `json:"work_no"`
	ProjectID int64                  `json:"project_id"`
	Rows      []DashboardStatusCount `json:"rows"`
}

// DashboardPersonRow 责任人统计行。
type DashboardPersonRow struct {
	Role   string `json:"role"`
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// DashboardPersonGroup 责任人分组。
type DashboardPersonGroup struct {
	UserID          string               `json:"userid"`
	Name            string               `json:"name"`
	SampleProjectID int64                `json:"sample_project_id"`
	Rows            []DashboardPersonRow `json:"rows"`
}

// DashboardSummary 总览看板汇总。
type DashboardSummary struct {
	Year           string                  `json:"year"`
	Years          []string                `json:"years"`
	ProjectCount   int                     `json:"project_count"`
	ProjectSummary DashboardProjectSummary `json:"project_summary"`
	ProjectPie     []DashboardStatusCount  `json:"project_pie"`
	ByWorkNo       []DashboardWorkNoGroup  `json:"by_work_no"`
	ByPerson       []DashboardPersonGroup  `json:"by_person"`
}

var dashboardProjectStatusOrder = []string{
	ProjectStatusNotStarted,
	ProjectStatusInProgress,
	ProjectStatusCompleted,
}

var dashboardSubtaskStatusOrder = []string{
	ProjectStatusNotStarted,
	ProjectStatusInProgress,
	ProjectStatusOverdue,
	ProjectStatusCompleted,
}

const (
	dashboardPersonRoleManager   = "project_manager"
	dashboardPersonRoleSubOwner  = "subtask_owner"
	dashboardWorkNoPlaceholder   = "（无工番号）"
	dashboardPersonNameFallback  = "（未指定）"
)

type personKey struct {
	userid string
	name   string
}

func orderedProjectDashboardRows(bucket map[string]int) []DashboardStatusCount {
	rows := make([]DashboardStatusCount, 0, len(bucket))
	for _, status := range dashboardProjectStatusOrder {
		count := bucket[status]
		if count <= 0 {
			continue
		}
		rows = append(rows, DashboardStatusCount{Status: status, Count: count})
	}
	return rows
}

func orderedSubtaskDashboardRows(bucket map[string]int) []DashboardStatusCount {
	rows := make([]DashboardStatusCount, 0, len(bucket))
	for _, status := range dashboardSubtaskStatusOrder {
		count := bucket[status]
		if count <= 0 {
			continue
		}
		rows = append(rows, DashboardStatusCount{Status: status, Count: count})
	}
	return rows
}

func orderedDashboardRows(bucket map[string]int) []DashboardStatusCount {
	return orderedSubtaskDashboardRows(bucket)
}

func incrementBucket(bucket map[string]int, status string) {
	if status == "" {
		return
	}
	bucket[status]++
}

func displayPersonName(userid, name string) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	userid = strings.TrimSpace(userid)
	if userid == "" {
		return dashboardPersonNameFallback
	}
	return userid
}

func makePersonKey(userid, name string) personKey {
	return personKey{userid: strings.TrimSpace(userid), name: strings.TrimSpace(name)}
}

func personKeyIsEmpty(key personKey) bool {
	return key.userid == "" && key.name == ""
}

func collectProjectYears(projects []Project) []string {
	seen := make(map[string]struct{})
	for _, p := range projects {
		year := strings.TrimSpace(p.Year)
		if year == "" {
			continue
		}
		seen[year] = struct{}{}
	}
	years := make([]string, 0, len(seen))
	for year := range seen {
		years = append(years, year)
	}
	sort.Strings(years)
	return years
}

func filterProjectsByYear(projects []Project, year string) []Project {
	year = strings.TrimSpace(year)
	if year == "" {
		return projects
	}
	filtered := make([]Project, 0, len(projects))
	for _, p := range projects {
		if strings.TrimSpace(p.Year) == year {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func groupSubtasksByProject(subtasks []ProjectSubtask) map[int64][]ProjectSubtask {
	out := make(map[int64][]ProjectSubtask)
	for _, s := range subtasks {
		out[s.ProjectID] = append(out[s.ProjectID], s)
	}
	return out
}

// SummarizeDashboard 汇总总览看板数据；year 为空表示全部年度。
func SummarizeDashboard(db *sql.DB, year string) (DashboardSummary, error) {
	projects, err := ListProjects(db)
	if err != nil {
		return DashboardSummary{}, err
	}

	subtasks, err := ListAllProjectSubtasks(db)
	if err != nil {
		return DashboardSummary{}, err
	}
	subtasksByProject := groupSubtasksByProject(subtasks)

	years := collectProjectYears(projects)
	filtered := filterProjectsByYear(projects, year)

	result := DashboardSummary{
		Year:         strings.TrimSpace(year),
		Years:        years,
		ProjectCount: len(filtered),
	}

	projectBucket := make(map[string]int)
	workNoBuckets := make(map[string]map[string]int)
	workNoProjectID := make(map[string]int64)
	managerBuckets := make(map[personKey]map[string]int)
	subtaskOwnerBuckets := make(map[personKey]map[string]int)
	personProjectID := make(map[personKey]int64)

	for _, project := range filtered {
		projectSubtasks := subtasksByProject[project.ID]
		projectStatus := EffectiveProjectStatus(project)
		incrementBucket(projectBucket, projectStatus)

		workNo := strings.TrimSpace(project.WorkNo)
		if workNo == "" {
			workNo = dashboardWorkNoPlaceholder
		}
		if _, ok := workNoBuckets[workNo]; !ok {
			workNoBuckets[workNo] = make(map[string]int)
		}
		if _, ok := workNoProjectID[workNo]; !ok {
			workNoProjectID[workNo] = project.ID
		}

		managerKey := makePersonKey(project.ManagerUserID, project.ManagerName)
		if !personKeyIsEmpty(managerKey) {
			if _, ok := managerBuckets[managerKey]; !ok {
				managerBuckets[managerKey] = make(map[string]int)
			}
			if _, ok := personProjectID[managerKey]; !ok {
				personProjectID[managerKey] = project.ID
			}
		}

		for _, subtask := range projectSubtasks {
			status := EffectiveSubtaskStatus(subtask)
			incrementBucket(workNoBuckets[workNo], status)
			if !personKeyIsEmpty(managerKey) {
				incrementBucket(managerBuckets[managerKey], status)
			}

			ownerKey := makePersonKey(subtask.OwnerUserID, subtask.OwnerName)
			if !personKeyIsEmpty(ownerKey) {
				if _, ok := subtaskOwnerBuckets[ownerKey]; !ok {
					subtaskOwnerBuckets[ownerKey] = make(map[string]int)
				}
				incrementBucket(subtaskOwnerBuckets[ownerKey], status)
				if _, ok := personProjectID[ownerKey]; !ok {
					personProjectID[ownerKey] = project.ID
				}
			}
		}
	}

	result.ProjectSummary = DashboardProjectSummary{
		Total:      len(filtered),
		NotStarted: projectBucket[ProjectStatusNotStarted],
		InProgress: projectBucket[ProjectStatusInProgress],
		Overdue:    0,
		Completed:  projectBucket[ProjectStatusCompleted],
	}
	result.ProjectPie = orderedProjectDashboardRows(projectBucket)

	workNos := make([]string, 0, len(workNoBuckets))
	for workNo := range workNoBuckets {
		workNos = append(workNos, workNo)
	}
	sort.Slice(workNos, func(i, j int) bool {
		return strings.ToLower(workNos[i]) < strings.ToLower(workNos[j])
	})
	for _, workNo := range workNos {
		rows := orderedDashboardRows(workNoBuckets[workNo])
		if len(rows) == 0 {
			continue
		}
		result.ByWorkNo = append(result.ByWorkNo, DashboardWorkNoGroup{
			WorkNo:    workNo,
			ProjectID: workNoProjectID[workNo],
			Rows:      rows,
		})
	}

	merged := make(map[personKey]*DashboardPersonGroup)
	mergePersonRole := func(buckets map[personKey]map[string]int, role string) {
		for key, bucket := range buckets {
			group := merged[key]
			if group == nil {
				group = &DashboardPersonGroup{
					UserID:          key.userid,
					Name:            displayPersonName(key.userid, key.name),
					SampleProjectID: personProjectID[key],
				}
				merged[key] = group
			}
			for _, status := range dashboardSubtaskStatusOrder {
				count := bucket[status]
				if count <= 0 {
					continue
				}
				group.Rows = append(group.Rows, DashboardPersonRow{
					Role:   role,
					Status: status,
					Count:  count,
				})
			}
		}
	}
	mergePersonRole(managerBuckets, dashboardPersonRoleManager)
	mergePersonRole(subtaskOwnerBuckets, dashboardPersonRoleSubOwner)

	names := make([]string, 0, len(merged))
	for key := range merged {
		names = append(names, displayPersonName(key.userid, key.name))
	}
	sort.Slice(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	for _, name := range names {
		for key, group := range merged {
			if displayPersonName(key.userid, key.name) != name {
				continue
			}
			if len(group.Rows) > 0 {
				result.ByPerson = append(result.ByPerson, *group)
			}
			break
		}
	}

	return result, nil
}
