package db

import (
	"strings"
	"unicode"
)

// NormalizePersonName 用于「登录账户 ↔ 项目负责人」姓名比对：
// 去首尾空白、去掉中间所有空白（含全角空格），不做其它变换。
func NormalizePersonName(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.TrimSpace(s) {
		if unicode.IsSpace(r) || r == '\u3000' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// AuthUserIdentityName 取账户用于比对的姓名：优先 display_name，空则用 username。
func AuthUserIdentityName(user AuthUser) string {
	n := NormalizePersonName(user.DisplayName)
	if n != "" {
		return n
	}
	return NormalizePersonName(user.Username)
}

// IsAdminOrAbove 管理员或 root（超级管理员）。
func IsAdminOrAbove(user AuthUser) bool {
	return user.CanManageAccounts || user.IsSuperAdmin || user.Role == RoleAdmin || user.Role == RoleSuperAdmin
}

// CanEditOwnedProject 是否可改该项目（及其子任务）：
// 管理员/root 任意项目；普通用户仅当身份姓名与项目负责人姓名一致。
func CanEditOwnedProject(user AuthUser, managerName string) bool {
	if !user.CanEditProjects {
		return false
	}
	if IsAdminOrAbove(user) {
		return true
	}
	identity := AuthUserIdentityName(user)
	if identity == "" {
		return false
	}
	return identity == NormalizePersonName(managerName)
}
