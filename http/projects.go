package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
)

type projectPayload struct {
	ID            int64              `json:"id"`
	Year          string             `json:"year"`
	WorkNo        string             `json:"work_no"`
	Name          string             `json:"name"`
	ManagerUserID string             `json:"manager_userid"`
	ManagerName   string             `json:"manager_name"`
	GroupChat     string             `json:"group_chat"`
	GroupChatID   string             `json:"group_chat_id"`
	Status        string             `json:"status"`
	StartDate     string             `json:"start_date"`
	EndDate       string             `json:"end_date"`
	Tasks         string             `json:"tasks"`
	Members       []db.ProjectMember `json:"members"`
}

type projectView struct {
	db.Project
	Members                []db.ProjectMember `json:"members"`
	TaskSummary            string             `json:"task_summary"`
	SubtaskStartDate       string             `json:"subtask_start_date"`
	SubtaskEndDate         string             `json:"subtask_end_date"`
	SubtaskActualStartDate string             `json:"subtask_actual_start_date"`
	SubtaskActualEndDate   string             `json:"subtask_actual_end_date"`
	SubtaskCount           int                `json:"subtask_count"`
	SubtaskAllCompleted    bool               `json:"subtask_all_completed"`
	SubtaskAnyActualStart  bool               `json:"subtask_any_actual_start"`
	SubtaskAnyOverdue      bool               `json:"subtask_any_overdue"`
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

func projectToView(p db.Project, members []db.ProjectMember, stats db.ProjectSubtaskStats) projectView {
	if members == nil {
		members = []db.ProjectMember{}
	}
	return projectView{
		Project:                p,
		Members:                members,
		TaskSummary:            stats.TaskSummary,
		SubtaskStartDate:       stats.SubtaskStartDate,
		SubtaskEndDate:         stats.SubtaskEndDate,
		SubtaskActualStartDate: stats.SubtaskActualStartDate,
		SubtaskActualEndDate:   stats.SubtaskActualEndDate,
		SubtaskCount:           stats.SubtaskCount,
		SubtaskAllCompleted:    stats.SubtaskAllCompleted,
		SubtaskAnyActualStart:  stats.SubtaskAnyActualStart,
		SubtaskAnyOverdue:      stats.SubtaskAnyOverdue,
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

func loadProjectView(id int64) (projectView, error) {
	p, err := db.GetProject(sqlDB, id)
	if err != nil {
		return projectView{}, err
	}
	members, err := db.ListProjectMembers(sqlDB, id)
	if err != nil {
		return projectView{}, err
	}
	if err := db.SyncProjectMembersFromSubtasks(sqlDB, id); err != nil {
		return projectView{}, err
	}
	members, err = db.ListProjectMembers(sqlDB, id)
	if err != nil {
		return projectView{}, err
	}
	stats, err := db.SummarizeProjectSubtaskStats(sqlDB, id)
	if err != nil {
		return projectView{}, err
	}
	return projectToView(p, members, stats), nil
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
	idStr := r.URL.Query().Get("id")
	if idStr != "" {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"ok": false, "error": "请使用有效的 ?id=数字",
			})
			return
		}
		view, err := loadProjectView(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]interface{}{
				"ok": false, "error": "项目不存在",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok": true, "project": view,
		})
		return
	}

	projects, err := db.ListProjects(sqlDB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	views := make([]projectView, 0, len(projects))
	subtaskStats, err := db.SummarizeAllProjectSubtaskStats(sqlDB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	for _, p := range projects {
		if err := db.SyncProjectMembersFromSubtasks(sqlDB, p.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"ok": false, "error": err.Error(),
			})
			return
		}
		members, err := db.ListProjectMembers(sqlDB, p.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"ok": false, "error": err.Error(),
			})
			return
		}
		views = append(views, projectToView(p, members, subtaskStats[p.ID]))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "count": len(views), "projects": views,
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

	if err := db.ReplaceProjectMembers(sqlDB, id, p.Members); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	created, _ := loadProjectView(id)
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

	if err := db.ReplaceProjectMembers(sqlDB, p.ID, p.Members); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	updated, _ := loadProjectView(p.ID)
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
