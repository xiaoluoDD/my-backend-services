package db

import "testing"

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("secret", hash) {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword("wrong", hash) {
		t.Fatal("wrong password should not verify")
	}
}

func TestRolePermissions(t *testing.T) {
	if !RoleCanEditProjects(RoleUser) || !RoleCanEditProjects(RoleAdmin) || !RoleCanEditProjects(RoleSuperAdmin) {
		t.Fatal("edit projects expected for user/admin/super")
	}
	if RoleCanManageAccounts(RoleUser) {
		t.Fatal("user should not manage accounts")
	}
	if !RoleCanManageAccounts(RoleAdmin) || !RoleCanManageAccounts(RoleSuperAdmin) {
		t.Fatal("admin/super should manage accounts")
	}
}
