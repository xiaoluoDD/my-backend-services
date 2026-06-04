package main

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
	"github.com/xiaoluoDD/my-backend-services/internal/wecom"
)

func handleWecomNotifyProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 POST",
		})
		return
	}

	body, _ := io.ReadAll(r.Body)
	var req struct {
		ProjectID int64  `json:"project_id"`
		Content   string `json:"content"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.ProjectID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请提供 project_id",
		})
		return
	}

	project, err := db.GetProject(sqlDB, req.ProjectID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "项目不存在",
		})
		return
	}

	members, err := db.ListProjectMembers(sqlDB, req.ProjectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	msgID, recipients, content, err := wecom.NotifyProjectMembers(project, members, req.Content)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	toUsers := make([]string, len(recipients))
	for i, rc := range recipients {
		toUsers[i] = rc.UserID
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"msg":        "项目提醒已发送",
		"msgid":      msgID,
		"project_id": req.ProjectID,
		"to_users":   toUsers,
		"recipients": recipients,
		"count":      len(recipients),
		"content":    content,
	})
}
