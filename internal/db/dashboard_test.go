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
		if g.ProjectName != "网页新建无子任务" {
			t.Fatalf("project_name=%q, want 网页新建无子任务", g.ProjectName)
		}
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

func TestListDashboardPersonTasksRoleFilter(t *testing.T) {
	dir := t.TempDir()
	sqlDB, err := Open(filepath.Join(dir, "person-role.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	projectID, err := CreateProject(sqlDB, Project{
		Year:          "2026",
		WorkNo:        "ROLE-1",
		Name:          "角色过滤项目",
		ManagerUserID: "mgr1",
		ManagerName:   "项目经理",
		StartDate:     "2026-01-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateProjectSubtask(sqlDB, ProjectSubtask{
		ProjectID:   projectID,
		Content:     "成员参与的子任务",
		OwnerUserID: "mgr1",
		OwnerName:   "项目经理",
		Status:      "进行中",
		Members: []ProjectMember{
			{UserID: "mem1", Name: "子任务成员"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	mgrRows, err := ListDashboardPersonTasks(sqlDB, "mgr1", "项目经理", "", "2026", "project_manager")
	if err != nil {
		t.Fatal(err)
	}
	if len(mgrRows) != 1 {
		t.Fatalf("manager role rows=%d, want 1", len(mgrRows))
	}
	if mgrRows[0].Role != "project_manager" {
		t.Fatalf("manager role=%q", mgrRows[0].Role)
	}

	memberRows, err := ListDashboardPersonTasks(sqlDB, "mem1", "子任务成员", "", "2026", "subtask_member")
	if err != nil {
		t.Fatal(err)
	}
	if len(memberRows) != 1 {
		t.Fatalf("member role rows=%d, want 1", len(memberRows))
	}
	if memberRows[0].Role != "subtask_member" {
		t.Fatalf("member role=%q", memberRows[0].Role)
	}

	// 旧别名 subtask_owner 应等价于子任务成员过滤
	legacyRows, err := ListDashboardPersonTasks(sqlDB, "mem1", "子任务成员", "", "2026", "subtask_owner")
	if err != nil {
		t.Fatal(err)
	}
	if len(legacyRows) != 1 {
		t.Fatalf("legacy owner alias rows=%d, want 1", len(legacyRows))
	}

	// 仅当项目经理、不是成员时，用成员角色过滤应为空
	empty, err := ListDashboardPersonTasks(sqlDB, "mgr1", "项目经理", "", "2026", "subtask_member")
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("manager filtered as subtask_member rows=%d, want 0", len(empty))
	}

	summary, err := SummarizeDashboard(sqlDB, "2026")
	if err != nil {
		t.Fatal(err)
	}
	foundMember := false
	for _, g := range summary.ByPerson {
		if g.UserID != "mem1" && g.Name != "子任务成员" {
			continue
		}
		for _, row := range g.Rows {
			if row.Role == "subtask_member" {
				foundMember = true
			}
			if row.Role == "subtask_owner" {
				t.Fatalf("unexpected legacy role in summary: %+v", row)
			}
		}
	}
	if !foundMember {
		t.Fatal("subtask member missing from by_person summary")
	}
}
