package db

import (
	"path/filepath"
	"testing"
)

func TestSummarizeDashboardIncludesProjectWithoutSubtasks(t *testing.T) {
	dir := t.TempDir()
	sqlDB, err := Open(filepath.Join(dir, "dash.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	id, err := CreateProject(sqlDB, Project{
		Year:      "2026",
		WorkNo:    "WEB-NEW",
		Name:      "网页新建无子任务",
		ManagerUserID: "u1",
		ManagerName:   "张三",
		StartDate: "2026-07-20",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = id

	summary, err := SummarizeDashboard(sqlDB, "2026")
	if err != nil {
		t.Fatal(err)
	}
	if summary.ProjectCount < 1 {
		t.Fatalf("project_count=%d, want >= 1", summary.ProjectCount)
	}

	foundWork := false
	for _, g := range summary.ByWorkNo {
		if g.WorkNo != "WEB-NEW" {
			continue
		}
		foundWork = true
		if len(g.Rows) == 0 {
			t.Fatal("by_work_no rows empty")
		}
	}
	if !foundWork {
		t.Fatal("project missing from by_work_no")
	}

	foundPerson := false
	for _, g := range summary.ByPerson {
		if g.UserID != "u1" && g.Name != "张三" {
			continue
		}
		foundPerson = true
		if len(g.Rows) == 0 {
			t.Fatal("by_person rows empty")
		}
	}
	if !foundPerson {
		t.Fatal("manager missing from by_person")
	}
}
