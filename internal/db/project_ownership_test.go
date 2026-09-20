package db

import "testing"

func TestNormalizePersonName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"张三", "张三"},
		{"  张三  ", "张三"},
		{"张 三", "张三"},
		{"张\u3000三", "张三"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := NormalizePersonName(tc.in); got != tc.want {
			t.Fatalf("NormalizePersonName(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCanEditOwnedProject(t *testing.T) {
	owner := AuthUser{
		Username: "zhangsan", DisplayName: "张三", Role: RoleUser, CanEditProjects: true,
	}
	other := AuthUser{
		Username: "lisi", DisplayName: "李四", Role: RoleUser, CanEditProjects: true,
	}
	admin := AuthUser{
		Username: "admin1", DisplayName: "管理员甲", Role: RoleAdmin,
		CanEditProjects: true, CanManageAccounts: true,
	}
	root := AuthUser{
		Username: "root", DisplayName: "超级管理员", Role: RoleSuperAdmin,
		CanEditProjects: true, CanManageAccounts: true, IsSuperAdmin: true,
	}
	noEdit := AuthUser{
		Username: "guest", DisplayName: "张三", Role: RoleUser, CanEditProjects: false,
	}

	if !CanEditOwnedProject(owner, "张三") {
		t.Fatal("owner should edit own project")
	}
	if !CanEditOwnedProject(owner, " 张 三 ") {
		t.Fatal("owner should match with spaces stripped")
	}
	if CanEditOwnedProject(other, "张三") {
		t.Fatal("other user must not edit")
	}
	if !CanEditOwnedProject(admin, "张三") {
		t.Fatal("admin should edit any")
	}
	if !CanEditOwnedProject(root, "任何人") {
		t.Fatal("root should edit any")
	}
	if CanEditOwnedProject(noEdit, "张三") {
		t.Fatal("no edit role must not edit even if name matches")
	}

	// display_name 空时回退 username
	byUser := AuthUser{Username: "王五", DisplayName: "", Role: RoleUser, CanEditProjects: true}
	if !CanEditOwnedProject(byUser, "王五") {
		t.Fatal("should fall back to username")
	}
}
