package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
)

const (
	deepseekChatURL     = "https://api.deepseek.com/chat/completions"
	deepseekModel       = "deepseek-chat"
	assistantMaxRunes   = 800
	assistantHTTPTimout = 40 * time.Second
)

const assistantSystemPrompt = `你是项目管理看板上的使用助手，只回答这套系统怎么用。
规则：
- 只用中文回答操作问题和规则解释。
- 你不能修改任何数据，也不能在服务器上执行命令。不要声称已经保存、删除、重启或切换了版本。
- 不要编造页面上不存在的按钮。
- 不确定时直接说不确定，并建议让管理员看运维工具或变更记录。
系统要点：
- 总览看板有项目状态、部门准时率、相关责任人、子任务责任人、项目进度。
- 部门准时率：有计划完成日才统计。准时率 = 准时数 / (子任务数 - 未到期)，未到期不进分母。点部门名或子任务数看全部明细，点未到期、准时数、准时率看对应子任务。
- 相关责任人是项目负责人，子任务责任人是子任务成员。点姓名或数字可看明细。
- 普通账户只能修改自己负责的项目和子任务；管理员和 root 可以改任意项目。负责人按登录姓名匹配。
- 甘特图按成员第一个部门分组，部门是标题行。进度在甘特页填写。导出的 Excel 按状态着色，改日期或状态后色条会变。
- 全屏展示会自动滚动表格。预览地址和正式地址不是同一份页面，切换正式版需要管理员操作。`

func handleAssistant(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/assistant/status" && r.Method == http.MethodGet:
		assistantStatus(w, r)
	case r.URL.Path == "/api/assistant/settings" && r.Method == http.MethodPut:
		saveAssistantSettings(w, r)
	case r.URL.Path == "/api/assistant/ask" && r.Method == http.MethodPost:
		askAssistant(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请求方法不正确",
		})
	}
}

func assistantStatus(w http.ResponseWriter, r *http.Request) {
	key, err := db.GetDeepseekAPIKey(sqlDB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	resp := map[string]interface{}{
		"ok":    true,
		"bound": key != "",
	}
	if user, err := currentAuthUser(r); err == nil && user.CanManageAccounts && key != "" {
		resp["masked"] = db.MaskSecret(key)
	}
	writeJSON(w, http.StatusOK, resp)
}

func saveAssistantSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireManageAccounts(w, r); !ok {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "读取请求失败",
		})
		return
	}
	var payload struct {
		APIKey string `json:"api_key"`
		Clear  bool   `json:"clear"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请求体格式错误",
		})
		return
	}
	key := strings.TrimSpace(payload.APIKey)
	if payload.Clear {
		key = ""
	} else if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请填写 API Key",
		})
		return
	}
	if err := db.SaveDeepseekAPIKey(sqlDB, key); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	msg := "已绑定 DeepSeek"
	if key == "" {
		msg = "已清除绑定"
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "msg": msg, "bound": key != "",
	})
}

func askAssistant(w http.ResponseWriter, r *http.Request) {
	if _, err := currentAuthUser(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"ok": false, "error": "请先登录后再提问",
		})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "读取请求失败",
		})
		return
	}
	var payload struct {
		Question string `json:"question"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请求体格式错误",
		})
		return
	}
	question := strings.TrimSpace(payload.Question)
	if question == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请输入问题",
		})
		return
	}
	if utf8.RuneCountInString(question) > assistantMaxRunes {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "问题太长，请缩短到 800 字以内",
		})
		return
	}

	key, err := db.GetDeepseekAPIKey(sqlDB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "尚未绑定 DeepSeek API，请管理员在运维工具 → 本地数据中绑定",
		})
		return
	}

	answer, err := callDeepseek(key, question)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "answer": answer,
	})
}

func callDeepseek(apiKey, question string) (string, error) {
	payload := map[string]interface{}{
		"model": deepseekModel,
		"messages": []map[string]string{
			{"role": "system", "content": assistantSystemPrompt},
			{"role": "user", "content": question},
		},
		"temperature": 0.3,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, deepseekChatURL, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: assistantHTTPTimout}
	res, err := client.Do(req)
	if err != nil {
		return "", errString("连接 DeepSeek 失败，请检查服务器是否能访问 api.deepseek.com")
	}
	defer res.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", errString("读取 DeepSeek 响应失败")
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", errString("DeepSeek 返回内容无法解析")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg := strings.TrimSpace(parsed.Error.Message)
		if msg == "" {
			msg = "DeepSeek 请求失败"
		}
		if res.StatusCode == http.StatusUnauthorized {
			msg = "API Key 无效，请在运维工具中重新绑定"
		}
		return "", errString(msg)
	}
	if len(parsed.Choices) == 0 {
		return "", errString("DeepSeek 没有返回回答")
	}
	answer := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if answer == "" {
		return "", errString("DeepSeek 没有返回回答")
	}
	return answer, nil
}

type errString string

func (e errString) Error() string { return string(e) }
