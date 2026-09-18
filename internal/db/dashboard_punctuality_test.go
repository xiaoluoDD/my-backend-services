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
		ProjectID:      projectID,
		Content:        "研发准时",
		OwnerUserID:    "dev1",
		OwnerName:      "研发甲",
		PlannedEndDate: earlier,
		ActualEndDate:  past,
		Members:        []ProjectMember{{UserID: "dev1", Name: "研发甲"}},
	}); err != nil {
		t.Fatal(err)
	}

	// 2. 市场：逾期未完结
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

	// 3. 双部门：延期完结
	if _, err := CreateProjectSubtask(sqlDB, ProjectSubtask{
		ProjectID:      projectID,
		Content:        "双部门延期完结",
		OwnerUserID:    "dev1",
		OwnerName:      "研发甲",
		PlannedEndDate: past,
		ActualEndDate:  earlier,
		Members:        []ProjectMember{{UserID: "both1", Name: "双部门"}},
	}); err != nil {
		t.Fatal(err)
	}

	// 4. 研发：未到期（计入总数与未到期，不影响准时率分母）
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

	// 5. 无部门逾期
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
	// 准时1 + 双部门延期1 + 未到期1 = total 3；not_due 1；on_time 1；rate=1/2=50
	if rd.Total != 3 || rd.NotDue != 1 || rd.OnTime != 1 {
		t.Fatalf("研发部 total=%d not_due=%d on_time=%d, want 3/1/1", rd.Total, rd.NotDue, rd.OnTime)
	}
	if rd.Rate != 50 {
		t.Fatalf("研发部 rate=%v, want 50", rd.Rate)
	}

	mk, ok := byName["市场部"]
	if !ok {
		t.Fatalf("missing 市场部")
	}
	if mk.Total != 2 || mk.NotDue != 0 || mk.OnTime != 0 || mk.Rate != 0 {
		t.Fatalf("市场部 total=%d not_due=%d on_time=%d rate=%v, want 2/0/0/0", mk.Total, mk.NotDue, mk.OnTime, mk.Rate)
	}

	unassigned, ok := byName[dashboardDeptUnassigned]
	if !ok {
		t.Fatalf("missing 未分配部门")
	}
	if unassigned.Total != 1 || unassigned.NotDue != 0 || unassigned.OnTime != 0 {
		t.Fatalf("未分配 total=%d not_due=%d on_time=%d, want 1/0/0", unassigned.Total, unassigned.NotDue, unassigned.OnTime)
	}
}

func TestClassifySubtaskPunctuality(t *testing.T) {
	today := time.Now()
	past := today.AddDate(0, 0, -3).Format("2006-01-02")
	future := today.AddDate(0, 0, 3).Format("2006-01-02")
	same := today.Format("2006-01-02")

	cases := []struct {
		name            string
		planned, actual string
		want            punctualityClass
	}{
		{"no plan", "", "", punctualitySkip},
		{"on time complete", past, past, punctualityOnTime},
		{"late complete", past, same, punctualityLate},
		{"overdue open", past, "", punctualityLate},
		{"not due open", future, "", punctualityNotDue},
	}
	for _, tc := range cases {
		got := classifySubtaskPunctuality(ProjectSubtask{
			PlannedEndDate: tc.planned,
			ActualEndDate:  tc.actual,
		})
		if got != tc.want {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
