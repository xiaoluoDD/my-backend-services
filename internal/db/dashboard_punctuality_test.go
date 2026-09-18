package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDepartmentPunctualitySummary(t *testing.T) {
	dir := t.TempDir()
	sqlDB, err := Open(filepath.Join(dir, "punctuality.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	mapping, err := UpsertWecomDepartments(sqlDB, []WecomDepartmentInput{
		{ID: 10, Name: "研发部"},
		{ID: 20, Name: "市场部"},
	})
	if err != nil {
		t.Fatal(err)
	}
	rdID := mapping[10]
	mkID := mapping[20]

	if err := ReplaceAppUsers(sqlDB, []AppUser{
		{UserID: "dev1", Name: "研发甲", DepartmentIDs: []int64{rdID}, Sources: "party:10"},
		{UserID: "mkt1", Name: "市场乙", DepartmentIDs: []int64{mkID}, Sources: "party:20"},
		{UserID: "both1", Name: "双部门", DepartmentIDs: []int64{rdID, mkID}, Sources: "party:10"},
	}); err != nil {
		t.Fatal(err)
	}

	today := time.Now()
	past := today.AddDate(0, 0, -10).Format("2006-01-02")
	earlier := today.AddDate(0, 0, -5).Format("2006-01-02")
	future := today.AddDate(0, 0, 10).Format("2006-01-02")

	projectID, err := CreateProject(sqlDB, Project{
		Year:          "2026",
		WorkNo:        "PUNC-1",
		Name:          "准时率测试项目",
		ManagerUserID: "dev1",
		ManagerName:   "研发甲",
		StartDate:     past,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1. 研发：准时完结
	if _, err := CreateProjectSubtask(sqlDB, ProjectSubtask{
		ProjectID:        projectID,
		Content:          "研发准时",
		OwnerUserID:      "dev1",
		OwnerName:        "研发甲",
		PlannedEndDate:   earlier,
		ActualEndDate:    past, // past <= earlier? past is -10, earlier is -5, so actual BEFORE planned → on time
		Members:          []ProjectMember{{UserID: "dev1", Name: "研发甲"}},
	}); err != nil {
		t.Fatal(err)
	}

	// 2. 市场：逾期未完结（计入不准时）
	if _, err := CreateProjectSubtask(sqlDB, ProjectSubtask{
		ProjectID:      projectID,
		Content:        "市场逾期",
		OwnerUserID:    "dev1",
		OwnerName:      "研发甲",
		PlannedEndDate: past,
		Members:        []ProjectMember{{UserID: "mkt1", Name: "市场乙"}},
	}); err != nil {
		t.Fatal(err)
	}

	// 3. 双部门：完结但延期（两部门各计不准时）
	if _, err := CreateProjectSubtask(sqlDB, ProjectSubtask{
		ProjectID:      projectID,
		Content:        "双部门延期完结",
		OwnerUserID:    "dev1",
		OwnerName:      "研发甲",
		PlannedEndDate: past,
		ActualEndDate:  earlier, // earlier > past → late
		Members:        []ProjectMember{{UserID: "both1", Name: "双部门"}},
	}); err != nil {
		t.Fatal(err)
	}

	// 4. 未到期未完结：不计入
	if _, err := CreateProjectSubtask(sqlDB, ProjectSubtask{
		ProjectID:      projectID,
		Content:        "未到期",
		OwnerUserID:    "dev1",
		OwnerName:      "研发甲",
		PlannedEndDate: future,
		Members:        []ProjectMember{{UserID: "dev1", Name: "研发甲"}},
	}); err != nil {
		t.Fatal(err)
	}

	// 5. 无部门成员：计入未分配
	if _, err := CreateProjectSubtask(sqlDB, ProjectSubtask{
		ProjectID:      projectID,
		Content:        "无部门逾期",
		OwnerUserID:    "dev1",
		OwnerName:      "研发甲",
		PlannedEndDate: past,
		Members:        []ProjectMember{{UserID: "ghost", Name: "幽灵"}},
	}); err != nil {
		t.Fatal(err)
	}

	summary, err := SummarizeDashboard(sqlDB, "2026")
	if err != nil {
		t.Fatal(err)
	}

	byName := map[string]DashboardDeptPunctuality{}
	for _, row := range summary.ByDepartmentPunctuality {
		byName[row.DepartmentName] = row
	}

	rd, ok := byName["研发部"]
	if !ok {
		t.Fatalf("missing 研发部: %+v", summary.ByDepartmentPunctuality)
	}
	// 研发：准时1 + 双部门延期1 = total 2, on_time 1 → 50%
	if rd.Total != 2 || rd.OnTime != 1 {
		t.Fatalf("研发部 total=%d on_time=%d, want 2/1", rd.Total, rd.OnTime)
	}
	if rd.Rate != 50 {
		t.Fatalf("研发部 rate=%v, want 50", rd.Rate)
	}

	mk, ok := byName["市场部"]
	if !ok {
		t.Fatalf("missing 市场部")
	}
	// 市场：逾期1 + 双部门延期1 = total 2, on_time 0 → 0%
	if mk.Total != 2 || mk.OnTime != 0 || mk.Rate != 0 {
		t.Fatalf("市场部 total=%d on_time=%d rate=%v, want 2/0/0", mk.Total, mk.OnTime, mk.Rate)
	}

	unassigned, ok := byName[dashboardDeptUnassigned]
	if !ok {
		t.Fatalf("missing 未分配部门")
	}
	if unassigned.Total != 1 || unassigned.OnTime != 0 {
		t.Fatalf("未分配 total=%d on_time=%d, want 1/0", unassigned.Total, unassigned.OnTime)
	}

	// 排序：准时率升序 → 市场(0) 应在 研发(50) 前；未分配(0) 与市场同率按名称
	if len(summary.ByDepartmentPunctuality) < 2 {
		t.Fatalf("rows=%d", len(summary.ByDepartmentPunctuality))
	}
	if summary.ByDepartmentPunctuality[0].Rate > summary.ByDepartmentPunctuality[1].Rate {
		t.Fatalf("expected ascending rate order, got %+v", summary.ByDepartmentPunctuality)
	}
}

func TestClassifySubtaskPunctuality(t *testing.T) {
	today := time.Now()
	past := today.AddDate(0, 0, -3).Format("2006-01-02")
	future := today.AddDate(0, 0, 3).Format("2006-01-02")
	same := today.Format("2006-01-02")

	cases := []struct {
		name              string
		planned, actual   string
		wantInc, wantTime bool
	}{
		{"no plan", "", "", false, false},
		{"on time complete", past, past, true, true},
		{"late complete", past, same, true, false},
		{"overdue open", past, "", true, false},
		{"not due open", future, "", false, false},
	}
	for _, tc := range cases {
		inc, ot := classifySubtaskPunctuality(ProjectSubtask{
			PlannedEndDate: tc.planned,
			ActualEndDate:  tc.actual,
		})
		if inc != tc.wantInc || ot != tc.wantTime {
			t.Fatalf("%s: got included=%v onTime=%v, want %v/%v", tc.name, inc, ot, tc.wantInc, tc.wantTime)
		}
	}
}
