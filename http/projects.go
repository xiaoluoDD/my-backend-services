package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
)

type projectPayload struct {
	ID            int64  `json:"id"`
	Year          string `json:"year"`
	WorkNo        string `json:"work_no"`
	Name          string `json:"name"`
	ManagerUserID string `json:"manager_userid"`
	ManagerName   string `json:"manager_name"`
	GroupChat     string `json:"group_chat"`
	GroupChatID   string `json:"group_chat_id"`
	Status        string `json:"status"`
	StartDate     string `json:"start_date"`
	EndDate       string `json:"end_date"`
	Tasks         string `json:"tasks"`
}

func (p projectPayload) toModel() db.Project {
	return db.Project{
		ID:            p.ID,
		Year:          p.Year,
		WorkNo:        p.WorkNo,
		Name:          p.Name,
		ManagerUserID: p.ManagerUserID,
		ManagerName:   p.ManagerName,
		GroupChat:     p.GroupChat,
		GroupChatID:   p.GroupChatID,
		Status:        p.Status,
		StartDate:     p.StartDate,
		EndDate:       p.EndDate,
		Tasks:         p.Tasks,
	}
}

func decodeProjectPayload(r *http.Request) (projectPayload, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return projectPayload{}, err
	}
	var p projectPayload
	if len(body) == 0 {
		return p, nil
	}
	err = json.Unmarshal(body, &p)
	return p, err
}

func handleProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listProjects(w, r)
	case http.MethodPost:
		createProject(w, r)
	case http.MethodPut:
		updateProject(w, r)
	case http.MethodDelete:
		deleteProject(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 GET / POST / PUT / DELETE",
		})
	}
}

func listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := db.ListProjects(sqlDB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "count": len(projects), "projects": projects,
	})
}

func createProject(w http.ResponseWriter, r *http.Request) {
	p, err := decodeProjectPayload(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请求体格式错误",
		})
		return
	}
	if p.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "项目名称不能为空",
		})
		return
	}

	id, err := db.CreateProject(sqlDB, p.toModel())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	created, _ := db.GetProject(sqlDB, id)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "msg": "项目已创建", "project": created,
	})
}

func updateProject(w http.ResponseWriter, r *http.Request) {
	p, err := decodeProjectPayload(r)
	if err != nil || p.ID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请提供有效的项目 id",
		})
		return
	}
	if p.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "项目名称不能为空",
		})
		return
	}

	if err := db.UpdateProject(sqlDB, p.toModel()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	updated, _ := db.GetProject(sqlDB, p.ID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "msg": "项目已更新", "project": updated,
	})
}

func deleteProject(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请使用 ?id=数字",
		})
		return
	}

	if err := db.DeleteProject(sqlDB, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "msg": "项目已删除", "id": id,
	})
}
