package db

import (
	"path/filepath"
	"sort"
	"testing"
)

// TestMultiDepartmentSync 覆盖"一人多部门"改造的核心行为：
// 企业微信部门导入、成员多部门归属同步与保留策略、按部门查成员、
// 项目成员多部门名称展示、手动编辑、部门删除级联、以及手动新增/改名部门被拒绝。
func TestMultiDepartmentSync(t *testing.T) {
	dir := t.TempDir()
	sqlDB, err := Open(filepath.Join(dir, "verify.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer sqlDB.Close()

	// 1. 导入两个企业微信部门
	mapping, err := UpsertWecomDepartments(sqlDB, []WecomDepartmentInput{
		{ID: 100, Name: "研发部"},
		{ID: 200, Name: "市场部"},
	})
	if err != nil {
		t.Fatalf("upsert depts: %v", err)
	}
	rdID := mapping[100]
	mkID := mapping[200]
	if rdID == 0 || mkID == 0 {
		t.Fatalf("mapping incomplete: %+v", mapping)
	}

	// 2. 同步一个属于两个部门的成员
	if err := ReplaceAppUsers(sqlDB, []AppUser{
		{UserID: "zhangsan", Name: "张三", DepartmentIDs: []int64{rdID, mkID}, Sources: "party:100"},
	}); err != nil {
		t.Fatalf("replace users: %v", err)
	}

	users, err := ListActiveUsers(sqlDB)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	u := users[0]
	if len(u.DepartmentIDs) != 2 {
		t.Fatalf("expected 2 department ids, got %v", u.DepartmentIDs)
	}
	t.Logf("department_name=%q department_ids=%v", u.DepartmentName, u.DepartmentIDs)

	// 3. 按部门查成员：两个部门都应该查到张三
	rdUsers, err := ListUsersByDepartmentID(sqlDB, rdID)
	if err != nil || len(rdUsers) != 1 {
		t.Fatalf("list by rd dept failed: %v users=%v", err, rdUsers)
	}
	mkUsers, err := ListUsersByDepartmentID(sqlDB, mkID)
	if err != nil || len(mkUsers) != 1 {
		t.Fatalf("list by mk dept failed: %v users=%v", err, mkUsers)
	}

	// 4. 再次同步（模拟第二次点击"同步成员"），部门不变，应保持一致，不重复
	if err := ReplaceAppUsers(sqlDB, []AppUser{
		{UserID: "zhangsan", Name: "张三", DepartmentIDs: []int64{rdID, mkID}, Sources: "party:100"},
	}); err != nil {
		t.Fatalf("replace users again: %v", err)
	}
	users, _ = ListActiveUsers(sqlDB)
	if len(users[0].DepartmentIDs) != 2 {
		t.Fatalf("expected still 2 department ids after re-sync, got %v", users[0].DepartmentIDs)
	}

	// 5. 项目成员部门展示（多部门拼接）
	if _, err := sqlDB.Exec(`INSERT INTO projects (name, manager_userid, manager_name, updated_at) VALUES ('测试项目','zhangsan','张三','2026-01-01')`); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	var projectID int64
	if err := sqlDB.QueryRow(`SELECT id FROM projects WHERE name='测试项目'`).Scan(&projectID); err != nil {
		t.Fatalf("get project id: %v", err)
	}
	if err := ReplaceProjectMembers(sqlDB, projectID, []ProjectMember{{UserID: "zhangsan", Name: "张三"}}); err != nil {
		t.Fatalf("replace project members: %v", err)
	}
	members, err := ListExplicitProjectMembers(sqlDB, projectID)
	if err != nil {
		t.Fatalf("list explicit members: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 project member, got %d", len(members))
	}
	names := splitDeptNames(members[0].DepartmentName)
	sort.Strings(names)
	if len(names) != 2 || names[0] != "市场部" || names[1] != "研发部" {
		t.Fatalf("unexpected project member department display: %q", members[0].DepartmentName)
	}

	// 6. 同步找不到部门数据时（DepartmentIDs 为空），不应清空既有归属
	if err := ReplaceAppUsers(sqlDB, []AppUser{
		{UserID: "zhangsan", Name: "张三", DepartmentIDs: nil, Sources: "party:100"},
	}); err != nil {
		t.Fatalf("replace users empty depts: %v", err)
	}
	users, _ = ListActiveUsers(sqlDB)
	if len(users[0].DepartmentIDs) != 2 {
		t.Fatalf("expected departments preserved when sync has no dept data, got %v", users[0].DepartmentIDs)
	}

	// 7. 手动编辑：只保留一个部门
	updated, err := UpdateAppUser(sqlDB, "zhangsan", "13800000000", []int64{rdID})
	if err != nil {
		t.Fatalf("update app user: %v", err)
	}
	if len(updated.DepartmentIDs) != 1 || updated.DepartmentIDs[0] != rdID {
		t.Fatalf("expected only rd dept after manual update, got %v", updated.DepartmentIDs)
	}

	// 8. 删除部门后级联清理
	if err := DeleteDepartment(sqlDB, rdID); err != nil {
		t.Fatalf("delete department: %v", err)
	}
	final, err := GetAppUser(sqlDB, "zhangsan")
	if err != nil {
		t.Fatalf("get app user after delete dept: %v", err)
	}
	if len(final.DepartmentIDs) != 0 {
		t.Fatalf("expected no departments after deleting the only one, got %v", final.DepartmentIDs)
	}

	// 9. 手动新增部门应被拒绝
	if _, err := CreateDepartment(sqlDB, "手动部门"); err == nil {
		t.Fatalf("expected CreateDepartment to be rejected")
	}

	// 10. 企业微信部门不允许改名
	if err := UpdateDepartment(sqlDB, mkID, "改名测试"); err == nil {
		t.Fatalf("expected rename of wecom-linked department to be rejected")
	}
}

func splitDeptNames(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	cur := ""
	for _, r := range s {
		if r == '、' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
