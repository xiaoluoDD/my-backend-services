package wecom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Config struct {
	CorpID  string
	AgentID int
	Secret  string
	ToUser  string
}

type apiResp struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	MsgID   string `json:"msgid,omitempty"`
}

type tokenResp struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type sendReq struct {
	ToUser  string `json:"touser"`
	MsgType string `json:"msgtype"`
	AgentID int    `json:"agentid"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

func LoadConfig() (Config, error) {
	corpID := os.Getenv("WECOM_CORP_ID")
	secret := os.Getenv("WECOM_SECRET")
	toUser := os.Getenv("WECOM_TO_USER")
	agentStr := os.Getenv("WECOM_AGENT_ID")

	if corpID == "" || secret == "" || toUser == "" || agentStr == "" {
		return Config{}, fmt.Errorf(
			"缺少环境变量 WECOM_CORP_ID、WECOM_SECRET、WECOM_AGENT_ID、WECOM_TO_USER",
		)
	}

	agentID, err := strconv.Atoi(agentStr)
	if err != nil {
		return Config{}, fmt.Errorf("WECOM_AGENT_ID 必须是数字: %w", err)
	}

	return Config{
		CorpID:  corpID,
		AgentID: agentID,
		Secret:  secret,
		ToUser:  toUser,
	}, nil
}

func getToken(cfg Config) (string, error) {
	url := fmt.Sprintf(
		"https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		cfg.CorpID, cfg.Secret,
	)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tr tokenResp
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("解析 token 响应失败: %w", err)
	}

	if tr.ErrCode != 0 {
		return "", fmt.Errorf("gettoken 失败: errcode=%d errmsg=%s", tr.ErrCode, tr.ErrMsg)
	}

	return tr.AccessToken, nil
}

// SendText 向配置的成员发送文本消息。
func SendText(cfg Config, content string) (msgID string, err error) {
	token, err := getToken(cfg)
	if err != nil {
		return "", err
	}

	url := "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=" + token

	var req sendReq
	req.ToUser = cfg.ToUser
	req.MsgType = "text"
	req.AgentID = cfg.AgentID
	req.Text.Content = content

	data, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var sr apiResp
	if err := json.Unmarshal(body, &sr); err != nil {
		return "", fmt.Errorf("解析发送响应失败: %w, body=%s", err, body)
	}

	if sr.ErrCode != 0 {
		return "", fmt.Errorf("发送失败: errcode=%d errmsg=%s", sr.ErrCode, sr.ErrMsg)
	}

	return sr.MsgID, nil
}

// SendTest 发送默认测试文案。
func SendTest() (msgID string, err error) {
	cfg, err := LoadConfig()
	if err != nil {
		return "", err
	}

	content := "📢 项目提醒测试（来自项目看板）\n时间：" + time.Now().Format("2006-01-02 15:04:05")
	return SendText(cfg, content)
}
