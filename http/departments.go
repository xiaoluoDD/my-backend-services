package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
)

type departmentPayload struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func handleDepartments(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listDepartments(w, r)
	case http.MethodPost:
		createDepartment(w, r)
	case http.MethodPut:
		updateDepartment(w, r)
	case http.MethodDelete:
		deleteDepartment(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 GET / POST / PUT / DELETE",
		})
	}
}

func listDepartments(w http.ResponseWriter, r *http.Request) {
	withMembers := r.URL.Query().Get("with_members") == "1"
	views, err := db.ListDepartmentViews(sqlDB, withMembers)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"count":       len(views),
		"departments": views,
	})
}

func decodeDepartmentPayload(r *http.Request) (departmentPayload, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return departmentPayload{}, err
	}
	var p departmentPayload
	if len(body) == 0 {
		return p, nil
	}
	err = json.Unmarshal(body, &p)
	return p, err
}

func createDepartment(w http.ResponseWriter, r *http.Request) {
	p, err := decodeDepartmentPayload(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请求体格式错误",
		})
		return
	}
	id, err := db.CreateDepartment(sqlDB, p.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	d, _ := db.GetDepartment(sqlDB, id)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "msg": "部门已创建", "department": d,
	})
}

func updateDepartment(w http.ResponseWriter, r *http.Request) {
	p, err := decodeDepartmentPayload(r)
	if err != nil || p.ID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请提供有效的部门 id",
		})
		return
	}
	if err := db.UpdateDepartment(sqlDB, p.ID, p.Name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	d, _ := db.GetDepartment(sqlDB, p.ID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "msg": "部门已更新", "department": d,
	})
}

func deleteDepartment(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请提供有效的部门 id",
		})
		return
	}
	if err := db.DeleteDepartment(sqlDB, id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "msg": "部门已删除",
	})
}

func handleWecomUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listWecomUsers(w, r)
	case http.MethodPut:
		updateWecomUser(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 GET / PUT",
		})
	}
}

func listWecomUsers(w http.ResponseWriter, r *http.Request) {
	users, err := db.ListActiveUsers(sqlDB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	stats, _ := db.Stats(sqlDB)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":    true,
		"count": len(users),
		"users": users,
		"stats": stats,
	})
}

func updateWecomUser(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请求体格式错误",
		})
		return
	}
	var req struct {
		UserID       string `json:"userid"`
		Mobile       string `json:"mobile"`
		DepartmentID int64  `json:"department_id"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请提供 userid",
		})
		return
	}

	updated, err := db.UpdateAppUser(sqlDB, req.UserID, req.Mobile, req.DepartmentID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "msg": "成员信息已保存", "user": updated,
	})
}
