package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

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
		content = fmt.Sprintf(
			"📢 项目提醒【%s】\n工番号：%s\n状态：%s\n负责人：%s\n时间：%s",
			project.Name, project.WorkNo, project.Status, project.ManagerName,
			time.Now().Format("2006-01-02 15:04:05"),
		)
	}
	if content == "" {
		content = "📢 项目提醒（来自项目看板）\n时间：" + time.Now().Format("2006-01-02 15:04:05")
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
