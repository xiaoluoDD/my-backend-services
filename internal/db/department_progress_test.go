package db

import (
	"path/filepath"
	"testing"
)

func TestListDepartmentProgressUsesManagerAndMembers(t *testing.T) {
	dir := t.TempDir()
	sqlDB, err := Open(filepath.Join(dir, "dept-progress.db"))
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
		{UserID: "mgr1", Name: "经理", DepartmentIDs: []int64{rdID}, Sources: "party:10"},
		{UserID: "m1", Name: "成员甲", DepartmentIDs: []int64{mkID}, Sources: "party:20"},
	}); err != nil {
		t.Fatal(err)
	}

	projectID, err := CreateProject(sqlDB, Project{
		Year:          "2026",
		WorkNo:        "DP-1",
		Name:          "部门进度测试",
		ManagerUserID: "mgr1",
		ManagerName:   "经理",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ReplaceProjectMembers(sqlDB, projectID, []ProjectMember{
		{UserID: "m1", Name: "成员甲"},
	}); err != nil {
		t.Fatal(err)
	}

	result, err := ListDepartmentProgress(sqlDB, "2026")
	if err != nil {
		t.Fatal(err)
	}
	var row *ProjectDepartmentProgressRow
	for i := range result.Projects {
		if result.Projects[i].ProjectID == projectID {
			row = &result.Projects[i]
			break
		}
	}
	if row == nil {
		t.Fatalf("missing project %d in %+v", projectID, result.Projects)
	}
	if len(row.Departments) != 2 {
		t.Fatalf("departments=%+v, want 2", row.Departments)
	}

	if err := UpsertDepartmentProgress(sqlDB, projectID, rdID, 40, "admin"); err != nil {
		t.Fatal(err)
	}
	result, err = ListDepartmentProgress(sqlDB, "2026")
	if err != nil {
		t.Fatal(err)
	}
	row = nil
	for i := range result.Projects {
		if result.Projects[i].ProjectID == projectID {
			row = &result.Projects[i]
			break
		}
	}
	if row == nil {
		t.Fatal("project missing after upsert")
	}
	found := false
	for _, d := range row.Departments {
		if d.DepartmentID == rdID {
			found = true
			if d.Progress == nil || *d.Progress != 40 {
				t.Fatalf("研发进度=%v, want 40", d.Progress)
			}
		}
		if d.DepartmentID == mkID && d.Progress != nil {
			t.Fatalf("市场部尚未填写，progress 应为 nil")
		}
	}
	if !found {
		t.Fatal("missing 研发部 cell")
	}
}
