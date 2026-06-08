package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
)

func handleProjectSubtasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listProjectSubtasks(w, r)
	case http.MethodPost:
		createProjectSubtask(w, r)
	case http.MethodPut:
		updateProjectSubtask(w, r)
	case http.MethodDelete:
		deleteProjectSubtask(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 GET / POST / PUT / DELETE",
		})
	}
}

func decodeSubtaskPayload(r *http.Request) (db.ProjectSubtask, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return db.ProjectSubtask{}, err
	}
	var s db.ProjectSubtask
	if len(body) == 0 {
		return s, nil
	}
	err = json.Unmarshal(body, &s)
	return s, err
}

func listProjectSubtasks(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.ParseInt(r.URL.Query().Get("project_id"), 10, 64)
	if err != nil || projectID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请提供 project_id",
		})
		return
	}

	list, err := db.ListProjectSubtasks(sqlDB, projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	if list == nil {
		list = []db.ProjectSubtask{}
	}
	for i := range list {
		list[i].Status = db.EffectiveSubtaskStatus(list[i])
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "count": len(list), "project_id": projectID, "subtasks": list,
	})
}

func createProjectSubtask(w http.ResponseWriter, r *http.Request) {
	s, err := decodeSubtaskPayload(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请求体格式错误",
		})
		return
	}
	if s.ProjectID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请提供 project_id",
		})
		return
	}

	id, err := db.CreateProjectSubtask(sqlDB, s)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	created, _ := db.GetProjectSubtask(sqlDB, id)
	created.Status = db.EffectiveSubtaskStatus(created)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "msg": "子任务已创建", "subtask": created,
	})
}

func updateProjectSubtask(w http.ResponseWriter, r *http.Request) {
	s, err := decodeSubtaskPayload(r)
	if err != nil || s.ID <= 0 || s.ProjectID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请提供有效的 id 与 project_id",
		})
		return
	}
	if err := db.UpdateProjectSubtask(sqlDB, s); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	updated, _ := db.GetProjectSubtask(sqlDB, s.ID)
	updated.Status = db.EffectiveSubtaskStatus(updated)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "msg": "子任务已更新", "subtask": updated,
	})
}

func deleteProjectSubtask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请提供有效的 id",
		})
		return
	}
	if err := db.DeleteProjectSubtask(sqlDB, id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "msg": "子任务已删除", "id": id,
	})
}
