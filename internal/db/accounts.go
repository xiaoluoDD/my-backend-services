package db

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// 账户角色（可扩展）。
const (
	RoleUser       = "user"        // 普通：可增改项目
	RoleAdmin      = "admin"       // 管理员：普通 + 账户管理
	RoleSuperAdmin = "super_admin" // 超级管理员（代码写死 root）
)

// 代码写死的超级管理员（不入库）。
const (
	HardcodedSuperUsername = "root"
	HardcodedSuperPassword = "root"
)

var (
	ErrAccountNotFound   = errors.New("账户不存在")
	ErrAccountExists     = errors.New("用户名已存在")
	ErrInvalidCredential = errors.New("用户名或密码错误")
	ErrInvalidRole       = errors.New("无效的权限角色")
	ErrCannotModifyRoot  = errors.New("不能通过账户接口修改超级管理员")
)

// Account 业务账户（网页端登录）。
type Account struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	Role         string `json:"role"`
	PasswordHash string `json:"-"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// AuthSession 登录会话。
type AuthSession struct {
	Token     string `json:"token"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
}

// AuthUser 当前登录用户视图。
type AuthUser struct {
	Username            string `json:"username"`
	DisplayName         string `json:"display_name"`
	Role                string `json:"role"`
	CanEditProjects     bool   `json:"can_edit_projects"`
	CanManageAccounts   bool   `json:"can_manage_accounts"`
	IsSuperAdmin        bool   `json:"is_super_admin"`
}

func NormalizeAccountRole(role string) (string, error) {
	switch strings.TrimSpace(role) {
	case RoleUser, RoleAdmin:
		return strings.TrimSpace(role), nil
	default:
		return "", ErrInvalidRole
	}
}

func RoleCanEditProjects(role string) bool {
	switch role {
	case RoleUser, RoleAdmin, RoleSuperAdmin:
		return true
	default:
		return false
	}
}

func RoleCanManageAccounts(role string) bool {
	switch role {
	case RoleAdmin, RoleSuperAdmin:
		return true
	default:
		return false
	}
}

func RoleLabel(role string) string {
	switch role {
	case RoleUser:
		return "普通"
	case RoleAdmin:
		return "管理员"
	case RoleSuperAdmin:
		return "超级管理员"
	default:
		return role
	}
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := sha256.Sum256(append(salt, []byte(password)...))
	return "v1$" + hex.EncodeToString(salt) + "$" + hex.EncodeToString(sum[:]), nil
}

func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 3 || parts[0] != "v1" {
		return false
	}
	salt, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}
	sum := sha256.Sum256(append(salt, []byte(password)...))
	return subtle.ConstantTimeCompare(sum[:], want) == 1
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func AuthUserFromRole(username, displayName, role string) AuthUser {
	if displayName == "" {
		displayName = username
	}
	return AuthUser{
		Username:          username,
		DisplayName:       displayName,
		Role:              role,
		CanEditProjects:   RoleCanEditProjects(role),
		CanManageAccounts: RoleCanManageAccounts(role),
		IsSuperAdmin:      role == RoleSuperAdmin,
	}
}

// AuthenticateAccount 校验登录；root/root 走代码写死超级管理员。
func AuthenticateAccount(dbConn *sql.DB, username, password string) (AuthUser, error) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return AuthUser{}, ErrInvalidCredential
	}

	if username == HardcodedSuperUsername {
		if subtle.ConstantTimeCompare([]byte(password), []byte(HardcodedSuperPassword)) != 1 {
			return AuthUser{}, ErrInvalidCredential
		}
		return AuthUserFromRole(HardcodedSuperUsername, "超级管理员", RoleSuperAdmin), nil
	}

	acc, err := GetAccountByUsername(dbConn, username)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return AuthUser{}, ErrInvalidCredential
		}
		return AuthUser{}, err
	}
	if !VerifyPassword(password, acc.PasswordHash) {
		return AuthUser{}, ErrInvalidCredential
	}
	return AuthUserFromRole(acc.Username, acc.DisplayName, acc.Role), nil
}

func CreateSession(dbConn *sql.DB, user AuthUser, ttl time.Duration) (AuthSession, error) {
	token, err := newToken()
	if err != nil {
		return AuthSession{}, err
	}
	now := time.Now()
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	session := AuthSession{
		Token:     token,
		Username:  user.Username,
		Role:      user.Role,
		ExpiresAt: now.Add(ttl).Format(time.RFC3339),
		CreatedAt: now.Format(time.RFC3339),
	}
	_, err = dbConn.Exec(
		`INSERT INTO auth_sessions (token, username, role, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		session.Token, session.Username, session.Role, session.ExpiresAt, session.CreatedAt,
	)
	if err != nil {
		return AuthSession{}, err
	}
	return session, nil
}

func DeleteSession(dbConn *sql.DB, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	_, err := dbConn.Exec(`DELETE FROM auth_sessions WHERE token=?`, token)
	return err
}

func GetSessionUser(dbConn *sql.DB, token string) (AuthUser, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return AuthUser{}, ErrInvalidCredential
	}
	var username, role, expiresAt string
	err := dbConn.QueryRow(
		`SELECT username, role, expires_at FROM auth_sessions WHERE token=?`, token,
	).Scan(&username, &role, &expiresAt)
	if err == sql.ErrNoRows {
		return AuthUser{}, ErrInvalidCredential
	}
	if err != nil {
		return AuthUser{}, err
	}
	exp, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || time.Now().After(exp) {
		_ = DeleteSession(dbConn, token)
		return AuthUser{}, ErrInvalidCredential
	}

	displayName := username
	if username == HardcodedSuperUsername {
		displayName = "超级管理员"
	} else if acc, err := GetAccountByUsername(dbConn, username); err == nil {
		displayName = acc.DisplayName
		role = acc.Role
	}
	return AuthUserFromRole(username, displayName, role), nil
}

func ListAccounts(dbConn *sql.DB) ([]Account, error) {
	rows, err := dbConn.Query(
		`SELECT id, username, display_name, role, created_at, updated_at
		 FROM accounts ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Account, 0)
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Username, &a.DisplayName, &a.Role, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func GetAccountByUsername(dbConn *sql.DB, username string) (Account, error) {
	var a Account
	err := dbConn.QueryRow(
		`SELECT id, username, display_name, role, password_hash, created_at, updated_at
		 FROM accounts WHERE username=? COLLATE NOCASE`, strings.TrimSpace(username),
	).Scan(&a.ID, &a.Username, &a.DisplayName, &a.Role, &a.PasswordHash, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return Account{}, ErrAccountNotFound
	}
	if err != nil {
		return Account{}, err
	}
	return a, nil
}

func GetAccountByID(dbConn *sql.DB, id int64) (Account, error) {
	var a Account
	err := dbConn.QueryRow(
		`SELECT id, username, display_name, role, password_hash, created_at, updated_at
		 FROM accounts WHERE id=?`, id,
	).Scan(&a.ID, &a.Username, &a.DisplayName, &a.Role, &a.PasswordHash, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return Account{}, ErrAccountNotFound
	}
	if err != nil {
		return Account{}, err
	}
	return a, nil
}

func CreateAccount(dbConn *sql.DB, username, password, displayName, role string) (Account, error) {
	username = strings.TrimSpace(username)
	displayName = strings.TrimSpace(displayName)
	if username == "" {
		return Account{}, fmt.Errorf("用户名不能为空")
	}
	if password == "" {
		return Account{}, fmt.Errorf("密码不能为空")
	}
	if strings.EqualFold(username, HardcodedSuperUsername) {
		return Account{}, ErrCannotModifyRoot
	}
	role, err := NormalizeAccountRole(role)
	if err != nil {
		return Account{}, err
	}
	if displayName == "" {
		displayName = username
	}
	hash, err := HashPassword(password)
	if err != nil {
		return Account{}, err
	}
	now := time.Now().Format(time.RFC3339)
	res, err := dbConn.Exec(
		`INSERT INTO accounts (username, display_name, role, password_hash, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		username, displayName, role, hash, now, now,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return Account{}, ErrAccountExists
		}
		return Account{}, err
	}
	id, _ := res.LastInsertId()
	return Account{
		ID: id, Username: username, DisplayName: displayName, Role: role,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func UpdateAccount(dbConn *sql.DB, id int64, displayName, role, newPassword string) (Account, error) {
	acc, err := GetAccountByID(dbConn, id)
	if err != nil {
		return Account{}, err
	}
	if strings.EqualFold(acc.Username, HardcodedSuperUsername) {
		return Account{}, ErrCannotModifyRoot
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = acc.DisplayName
	}
	if role != "" {
		role, err = NormalizeAccountRole(role)
		if err != nil {
			return Account{}, err
		}
	} else {
		role = acc.Role
	}
	hash := acc.PasswordHash
	if strings.TrimSpace(newPassword) != "" {
		hash, err = HashPassword(strings.TrimSpace(newPassword))
		if err != nil {
			return Account{}, err
		}
	}
	now := time.Now().Format(time.RFC3339)
	_, err = dbConn.Exec(
		`UPDATE accounts SET display_name=?, role=?, password_hash=?, updated_at=? WHERE id=?`,
		displayName, role, hash, now, id,
	)
	if err != nil {
		return Account{}, err
	}
	acc.DisplayName = displayName
	acc.Role = role
	acc.PasswordHash = hash
	acc.UpdatedAt = now
	return acc, nil
}

// ChangeOwnPassword 当前登录用户修改自己的密码（无需原密码）。
func ChangeOwnPassword(dbConn *sql.DB, username, newPassword string) error {
	username = strings.TrimSpace(username)
	newPassword = strings.TrimSpace(newPassword)
	if username == "" {
		return ErrInvalidCredential
	}
	if newPassword == "" {
		return fmt.Errorf("新密码不能为空")
	}
	if len(newPassword) < 4 {
		return fmt.Errorf("新密码至少 4 位")
	}
	if strings.EqualFold(username, HardcodedSuperUsername) {
		return ErrCannotModifyRoot
	}
	acc, err := GetAccountByUsername(dbConn, username)
	if err != nil {
		return err
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339)
	_, err = dbConn.Exec(
		`UPDATE accounts SET password_hash=?, updated_at=? WHERE id=?`,
		hash, now, acc.ID,
	)
	return err
}

func DeleteAccount(dbConn *sql.DB, id int64) error {
	acc, err := GetAccountByID(dbConn, id)
	if err != nil {
		return err
	}
	if strings.EqualFold(acc.Username, HardcodedSuperUsername) {
		return ErrCannotModifyRoot
	}
	_, err = dbConn.Exec(`DELETE FROM accounts WHERE id=?`, id)
	if err != nil {
		return err
	}
	_, _ = dbConn.Exec(`DELETE FROM auth_sessions WHERE username=?`, acc.Username)
	return nil
}
