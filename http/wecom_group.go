package main

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
	"github.com/xiaoluoDD/my-backend-services/internal/wecom"
)

func handleWecomSendGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 POST",
		})
		return
	}

	body, _ := io.ReadAll(r.Body)
	var req struct {
		ProjectID int64  `json:"project_id"`
		ChatID    string `json:"chatid"`
		Content   string `json:"content"`
	}
	_ = json.Unmarshal(body, &req)

	chatid := req.ChatID
	var project db.Project

	if req.ProjectID > 0 {
		p, err := db.GetProject(sqlDB, req.ProjectID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"ok": false, "error": "项目不存在",
			})
			return
		}
		project = p
		if chatid == "" {
			chatid = project.GroupChatID
		}
	}

	if chatid == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请填写群聊 chatid，或在项目中配置 group_chat_id",
		})
		return
	}

	content := req.Content
	if content == "" && project.ID > 0 {
		content = wecom.FormatProjectReminder(project, "")
	}
	if content == "" {
		content = "📢 项目提醒（来自项目看板）"
	}

	if err := wecom.SendAppChatText(chatid, content); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"msg":     "群消息已发送",
		"chatid":  chatid,
		"content": content,
	})
}
