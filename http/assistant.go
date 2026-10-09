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

const assistantSystemPrompt = `你是这套「项目管理看板」整个软件的使用助手，不是只看总览看板。
用户经常停在总览看板里提问，但问题可以涉及任意页面。总览看板上没有的按钮，不代表软件没有这个功能。回答时按下面的全站说明，告诉用户去哪个界面、点哪个按钮。
规则：
- 只用中文回答操作问题和规则解释。
- 你不能修改任何数据，也不能在服务器上执行命令。不要声称已经保存、删除、重启或切换了版本。
- 下面写到的功能都已经有了。不要说系统没有。
- 用户说「新增任务」「加任务」时，先分清两种：新建项目，或给已有项目加子任务。两种都要说清入口。
- 没写到的按钮不要编造。不确定时直接说不确定。

全站界面（左上角下拉切换）：
1. 总览看板：项目状态、部门准时率、相关责任人、子任务责任人、项目进度。可按年度筛选。右上角「问一下」只回答问题。全屏展示会自动滚动表格，电视上用来展示，这里不能新增。
2. 项目界面：项目列表。顶栏有「筛选」和「新增项目」（需登录且有编辑权限）。点项目卡片进入项目详情。
3. 部门管理、成员管理、账户管理、运维工具：管理员用。普通用户通常看不到。

新增项目：
- 切到「项目界面」，点顶栏「新增项目」。
- 填写年度、工番号、项目名称、负责人部门、项目负责人、项目成员、项目启动日期、实际完结日期。
- 普通用户新建时，负责人必须是自己。管理员和 root 可以指定别人。

项目详情：
- 按钮有「编辑项目」「标记完结」「查看子任务」「导出甘特」「删除项目」。
- 编辑、完结、删除仅项目负责人或管理员可用。
- 有未完成子任务时，不能填写项目实际完结日期。

新增子任务（这就是项目里的任务）：
- 打开项目，点「查看子任务」。
- 单条：页头「新增子任务」。字段有任务内容、负责人（固定为该项目负责人）、筛选部门、子项目成员、任务状态、计划开始、实际开始、计划完成、实际完成、备注。
- 批量：电脑端页头「批量新增」。表格列是任务内容、成员、计划开始、计划完成、实际开始、实际完成、备注。负责人固定为项目负责人。空行跳过，单次最多 50 条，可「再加一行」。在任务内容格粘贴多行会按行拆开。填完点「批量创建」。手机端没有批量新增。
- 只有该项目负责人或管理员能新增、编辑、删除子任务。

部门准时率：
- 有计划完成日才统计。准时率 = 准时数 / (子任务数 - 未到期)，未到期不进分母。
- 点部门名或子任务数看全部明细，点未到期、准时数、准时率看对应子任务。
- 相关责任人是项目负责人，子任务责任人是子任务成员。点姓名或数字可看明细。

甘特图：
- 项目详情点「导出甘特」。按成员第一个部门分组，部门是标题行。
- 左侧是任务、状态、责任人、进度、计划开始、计划结束、天数。进度在甘特页填写，并随 Excel 导出。
- 可「导出 Excel」「打印 / 导出 PDF」。Excel 按状态着色，改日期或状态后色条会变。

权限：
- 普通账户只能改自己负责的项目及其子任务，登录姓名须与项目负责人一致。
- 管理员和 root 可改任意项目，也可管理账户。`

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
			{"role": "user", "content": "请按整个软件回答，不要只根据总览看板判断有没有这个功能。问题：" + question},
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
