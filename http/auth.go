package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
)

func bearerToken(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if h == "" {
		return strings.TrimSpace(r.URL.Query().Get("token"))
	}
	const prefix = "Bearer "
	if strings.HasPrefix(h, prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

func currentAuthUser(r *http.Request) (db.AuthUser, error) {
	token := bearerToken(r)
	if token == "" {
		return db.AuthUser{}, db.ErrInvalidCredential
	}
	return db.GetSessionUser(sqlDB, token)
}

func requireManageAccounts(w http.ResponseWriter, r *http.Request) (db.AuthUser, bool) {
	user, err := currentAuthUser(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"ok": false, "error": "请先登录",
		})
		return db.AuthUser{}, false
	}
	if !user.CanManageAccounts {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{
			"ok": false, "error": "需要管理员权限",
		})
		return db.AuthUser{}, false
	}
	return user, true
}

// requireEditProjects 校验可编辑项目权限。
// 无 Token 时兼容桌面端 Qt（暂不强制登录）；有 Token 则必须具备编辑权限。
func requireEditProjects(w http.ResponseWriter, r *http.Request) bool {
	token := bearerToken(r)
	if token == "" {
		return true
	}
	user, err := currentAuthUser(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"ok": false, "error": "登录已失效，请重新登录",
		})
		return false
	}
	if !user.CanEditProjects {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{
			"ok": false, "error": "当前账户无权修改项目",
		})
		return false
	}
	return true
}

func handleAuth(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/auth")
	path = strings.Trim(path, "/")
	switch {
	case path == "login" && r.Method == http.MethodPost:
		handleAuthLogin(w, r)
	case path == "logout" && r.Method == http.MethodPost:
		handleAuthLogout(w, r)
	case path == "me" && r.Method == http.MethodGet:
		handleAuthMe(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"ok": false, "error": "未知接口，请使用 /api/auth/login|logout|me",
		})
	}
}

func handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请求 JSON 无效",
		})
		return
	}

	user, err := db.AuthenticateAccount(sqlDB, req.Username, req.Password)
	if err != nil {
		if errors.Is(err, db.ErrInvalidCredential) {
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"ok": false, "error": "用户名或密码错误",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	session, err := db.CreateSession(sqlDB, user, 7*24*time.Hour)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"token":   session.Token,
		"user":    user,
		"expires": session.ExpiresAt,
	})
}

func handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	_ = db.DeleteSession(sqlDB, bearerToken(r))
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func handleAuthMe(w http.ResponseWriter, r *http.Request) {
	user, err := currentAuthUser(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"ok": false, "error": "未登录或登录已失效",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"user": user,
	})
}

func handleAccounts(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireManageAccounts(w, r); !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		listAccountsAPI(w, r)
	case http.MethodPost:
		createAccountAPI(w, r)
	case http.MethodPut:
		updateAccountAPI(w, r)
	case http.MethodDelete:
		deleteAccountAPI(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 GET / POST / PUT / DELETE",
		})
	}
}

func accountPublicView(a db.Account) map[string]interface{} {
	return map[string]interface{}{
		"id":           a.ID,
		"username":     a.Username,
		"display_name": a.DisplayName,
		"role":         a.Role,
		"role_label":   db.RoleLabel(a.Role),
		"created_at":   a.CreatedAt,
		"updated_at":   a.UpdatedAt,
	}
}

func listAccountsAPI(w http.ResponseWriter, r *http.Request) {
	list, err := db.ListAccounts(sqlDB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	views := make([]map[string]interface{}, 0, len(list)+1)
	// 超级管理员仅展示，不可通过接口改删。
	views = append(views, map[string]interface{}{
		"id":           0,
		"username":     db.HardcodedSuperUsername,
		"display_name": "超级管理员",
		"role":         db.RoleSuperAdmin,
		"role_label":   db.RoleLabel(db.RoleSuperAdmin),
		"builtin":      true,
		"created_at":   "",
		"updated_at":   "",
	})
	for _, a := range list {
		v := accountPublicView(a)
		v["builtin"] = false
		views = append(views, v)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"accounts": views,
	})
}

func createAccountAPI(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请求 JSON 无效",
		})
		return
	}
	if req.Role == "" {
		req.Role = db.RoleUser
	}
	acc, err := db.CreateAccount(sqlDB, req.Username, req.Password, req.DisplayName, req.Role)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, db.ErrAccountExists) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"account": accountPublicView(acc),
	})
}

func updateAccountAPI(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req struct {
		ID          int64  `json:"id"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		Password    string `json:"password"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.ID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请提供有效的账户 id",
		})
		return
	}
	acc, err := db.UpdateAccount(sqlDB, req.ID, req.DisplayName, req.Role, req.Password)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, db.ErrAccountNotFound) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"account": accountPublicView(acc),
	})
}

func deleteAccountAPI(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请使用 ?id=数字",
		})
		return
	}
	if err := db.DeleteAccount(sqlDB, id); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, db.ErrAccountNotFound) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
