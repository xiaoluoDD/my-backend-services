package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
)

func handleProjectSubtasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listProjectSubtasks(w, r)
	case http.MethodPost:
		if !requireEditProjects(w, r) {
			return
		}
		createProjectSubtask(w, r)
	case http.MethodPut:
		if !requireEditProjects(w, r) {
			return
		}
		updateProjectSubtask(w, r)
	case http.MethodDelete:
		if !requireEditProjects(w, r) {
			return
		}
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
	list, err = attachSubtaskMembers(projectID, list)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
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

	wasOnProject := projectMemberSnapshot(s.ProjectID)

	id, err := db.CreateProjectSubtask(sqlDB, s)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	// 子任务新增后若项目仍存在未完结子任务，则清空实际完结日期
	cleared, err := db.ReconcileProjectEndDate(sqlDB, s.ProjectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	if err := syncSubtaskMembersToProject(s.ProjectID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	created, err := loadSubtaskWithMembers(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	notifyNewSubtaskMembers(s.ProjectID, created, created.Members, wasOnProject)

	msg := "子任务已创建"
	if cleared {
		msg = "子任务已创建；项目原已完结，已清空实际完结日期并更新项目状态"
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":                         true,
		"msg":                        msg,
		"subtask":                    created,
		"project_completion_cleared": cleared,
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
	before, err := loadSubtaskWithMembers(s.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "子任务不存在",
		})
		return
	}
	wasOnProject := projectMemberSnapshot(s.ProjectID)
	addedMembers := addedSubtaskMembers(before.Members, s.Members)

	if err := db.UpdateProjectSubtask(sqlDB, s); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	removed := removedSubtaskMembers(before.Members, s.Members)
	if err := syncSubtaskMembersToProjectAfterChange(s.ProjectID, removed); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	cleared, err := db.ReconcileProjectEndDate(sqlDB, s.ProjectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	updated, err := loadSubtaskWithMembers(s.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	notifyNewSubtaskMembers(s.ProjectID, updated, addedMembers, wasOnProject)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "msg": map[bool]string{true: "子任务已更新；项目实际完结日期已按子任务状态重算", false: "子任务已更新"}[cleared], "subtask": updated,
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
	before, err := loadSubtaskWithMembers(id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	if err := db.DeleteProjectSubtask(sqlDB, id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	if err := syncSubtaskMembersToProjectAfterChange(before.ProjectID, before.Members); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	cleared, err := db.ReconcileProjectEndDate(sqlDB, before.ProjectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "msg": map[bool]string{true: "子任务已删除；项目实际完结日期已按子任务状态重算", false: "子任务已删除"}[cleared], "id": id,
	})
}

type batchSubtasksPayload struct {
	ProjectID int64              `json:"project_id"`
	Subtasks  []db.ProjectSubtask `json:"subtasks"`
}

const maxBatchSubtasks = 50

// handleProjectSubtasksBatch 批量新建子任务（Excel 表格式录入）。
// 空行（内容为空且无其他有效字段）自动跳过；有内容以外字段但内容为空则报错。
func handleProjectSubtasksBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 POST",
		})
		return
	}
	if !requireEditProjects(w, r) {
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "读取请求失败",
		})
		return
	}
	var payload batchSubtasksPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请求体格式错误",
		})
		return
	}
	if payload.ProjectID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请提供 project_id",
		})
		return
	}
	if len(payload.Subtasks) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请至少填写一行子任务",
		})
		return
	}
	if len(payload.Subtasks) > maxBatchSubtasks {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": fmt.Sprintf("单次最多新建 %d 条子任务", maxBatchSubtasks),
		})
		return
	}

	project, err := db.GetProject(sqlDB, payload.ProjectID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "项目不存在",
		})
		return
	}

	type preparedRow struct {
		rowIndex int
		subtask  db.ProjectSubtask
	}
	prepared := make([]preparedRow, 0, len(payload.Subtasks))
	for i, raw := range payload.Subtasks {
		content := strings.TrimSpace(raw.Content)
		hasExtra := strings.TrimSpace(raw.PlannedStartDate) != "" ||
			strings.TrimSpace(raw.PlannedEndDate) != "" ||
			strings.TrimSpace(raw.Remark) != "" ||
			len(raw.Members) > 0
		if content == "" {
			if hasExtra {
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{
					"ok": false, "error": fmt.Sprintf("第 %d 行：已填写其他字段，但任务内容不能为空", i+1),
				})
				return
			}
			continue
		}
		st := raw
		st.ID = 0
		st.ProjectID = payload.ProjectID
		st.Content = content
		// 批量新建时负责人固定为项目负责人（前端也会传，这里再兜底一次）
		if strings.TrimSpace(st.OwnerUserID) == "" && strings.TrimSpace(st.OwnerName) == "" {
			st.OwnerUserID = project.ManagerUserID
			st.OwnerName = project.ManagerName
		}
		st.ActualStartDate = ""
		st.ActualEndDate = ""
		// 成员只取有效 userid，通常 0～1 人
		members := make([]db.ProjectMember, 0, len(st.Members))
		seen := make(map[string]struct{})
		for _, m := range st.Members {
			uid := strings.TrimSpace(m.UserID)
			if uid == "" {
				continue
			}
			if _, ok := seen[uid]; ok {
				continue
			}
			seen[uid] = struct{}{}
			members = append(members, db.ProjectMember{
				UserID: uid,
				Name:   strings.TrimSpace(m.Name),
			})
		}
		st.Members = members
		prepared = append(prepared, preparedRow{rowIndex: i + 1, subtask: st})
	}
	if len(prepared) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "没有可创建的子任务（请至少填写一行任务内容）",
		})
		return
	}

	wasOnProject := projectMemberSnapshot(payload.ProjectID)
	created := make([]db.ProjectSubtask, 0, len(prepared))
	for _, item := range prepared {
		id, err := db.CreateProjectSubtask(sqlDB, item.subtask)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"ok":      false,
				"error":   fmt.Sprintf("第 %d 行创建失败：%s", item.rowIndex, err.Error()),
				"created": len(created),
			})
			return
		}
		full, err := loadSubtaskWithMembers(id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"ok":      false,
				"error":   fmt.Sprintf("第 %d 行创建后读取失败：%s", item.rowIndex, err.Error()),
				"created": len(created),
			})
			return
		}
		created = append(created, full)
	}

	cleared, err := db.ReconcileProjectEndDate(sqlDB, payload.ProjectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(), "created": len(created),
		})
		return
	}
	if err := syncSubtaskMembersToProject(payload.ProjectID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(), "created": len(created),
		})
		return
	}

	// 通知：按创建顺序发送；已通知过「加入项目」的成员记入快照，避免同批重复刷屏
	for _, st := range created {
		notifyNewSubtaskMembers(payload.ProjectID, st, st.Members, wasOnProject)
		for _, m := range st.Members {
			if m.UserID != "" {
				wasOnProject[m.UserID] = struct{}{}
			}
		}
	}

	msg := fmt.Sprintf("已批量创建 %d 条子任务", len(created))
	if cleared {
		msg += "；项目原已完结，已清空实际完结日期并更新项目状态"
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":                         true,
		"msg":                        msg,
		"count":                      len(created),
		"subtasks":                   created,
		"project_completion_cleared": cleared,
	})
}
