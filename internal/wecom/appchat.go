package wecom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/xiaoluoDD/my-backend-services/internal/logger"
)

type appChatSendReq struct {
	ChatID  string `json:"chatid"`
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
	Safe int `json:"safe"`
}

// SendAppChatText 向已有应用群聊发送文本（不创建群）。
func SendAppChatText(chatid, content string) error {
	if chatid == "" {
		return fmt.Errorf("未指定群聊 chatid")
	}
	if content == "" {
		return fmt.Errorf("消息内容不能为空")
	}

	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	log := logger.Default()
	log.Info("wecom appchat send", "chatid", chatid, "content_len", len(content))

	token, err := getToken(cfg)
	if err != nil {
		return err
	}

	url := "https://qyapi.weixin.qq.com/cgi-bin/appchat/send?access_token=" + token

	var req appChatSendReq
	req.ChatID = chatid
	req.MsgType = "text"
	req.Text.Content = content
	req.Safe = 0

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var sr apiResp
	if err := json.Unmarshal(body, &sr); err != nil {
		return fmt.Errorf("解析 appchat/send 响应失败: %w, body=%s", err, body)
	}
	if sr.ErrCode != 0 {
		log.Error("wecom appchat send api error", "errcode", sr.ErrCode, "errmsg", sr.ErrMsg)
		return fmt.Errorf("群消息发送失败: errcode=%d errmsg=%s", sr.ErrCode, sr.ErrMsg)
	}

	log.Info("wecom appchat send ok", "chatid", chatid)
	return nil
}
