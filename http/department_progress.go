package main

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
)

func handleDepartmentProgress(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listDepartmentProgress(w, r)
	case http.MethodPut:
		putDepartmentProgress(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 GET 或 PUT",
		})
	}
}

func listDepartmentProgress(w http.ResponseWriter, r *http.Request) {
	year := r.URL.Query().Get("year")
	result, err := db.ListDepartmentProgress(sqlDB, year)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	projects := result.Projects
	if projects == nil {
		projects = []db.ProjectDepartmentProgressRow{}
	}
	years := result.Years
	if years == nil {
		years = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"count":    len(projects),
		"projects": projects,
		"years":    years,
	})
}

func putDepartmentProgress(w http.ResponseWriter, r *http.Request) {
	user, ok := requireManageAccounts(w, r)
	if !ok {
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请求体格式错误",
		})
		return
	}
	var req struct {
		ProjectID    int64 `json:"project_id"`
		DepartmentID int64 `json:"department_id"`
		Progress     *int  `json:"progress"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.Progress == nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请提供 project_id、department_id、progress",
		})
		return
	}

	updatedBy := user.Username
	if display := user.DisplayName; display != "" {
		updatedBy = display
	}
	if err := db.UpsertDepartmentProgress(sqlDB, req.ProjectID, req.DepartmentID, *req.Progress, updatedBy); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":  true,
		"msg": "部门进度已保存",
	})
}
