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

	"github.com/xiaoluoDD/my-backend-services/internal/logger"
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

	if corpID == "" || secret == "" || agentStr == "" {
		return Config{}, fmt.Errorf(
			"缺少环境变量 WECOM_CORP_ID、WECOM_SECRET、WECOM_AGENT_ID",
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
	log := logger.Default()
	log.Debug("wecom gettoken start", "corp_id", cfg.CorpID, "agent_id", cfg.AgentID)

	url := fmt.Sprintf(
		"https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		cfg.CorpID, cfg.Secret,
	)

	resp, err := http.Get(url)
	if err != nil {
		log.Error("wecom gettoken request failed", "err", err)
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("wecom gettoken read body failed", "err", err)
		return "", err
	}

	var tr tokenResp
	if err := json.Unmarshal(body, &tr); err != nil {
		log.Error("wecom gettoken parse failed", "err", err, "body_len", len(body))
		return "", fmt.Errorf("解析 token 响应失败: %w", err)
	}

	if tr.ErrCode != 0 {
		log.Error("wecom gettoken api error", "errcode", tr.ErrCode, "errmsg", tr.ErrMsg)
		return "", fmt.Errorf("gettoken 失败: errcode=%d errmsg=%s", tr.ErrCode, tr.ErrMsg)
	}

	log.Debug("wecom gettoken ok", "expires_in", tr.ExpiresIn)
	return tr.AccessToken, nil
}

// SendText 向指定成员发送文本消息。toUser 为空时使用配置中的 WECOM_TO_USER。
func SendText(cfg Config, toUser, content string) (msgID string, err error) {
	if toUser == "" {
		toUser = cfg.ToUser
	}
	if toUser == "" {
		return "", fmt.Errorf("未指定接收人 userid")
	}

	log := logger.Default()
	log.Info("wecom send text",
		"to_user", toUser,
		"agent_id", cfg.AgentID,
		"content_len", len(content),
	)

	token, err := getToken(cfg)
	if err != nil {
		return "", err
	}

	url := "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=" + token

	var req sendReq
	req.ToUser = toUser
	req.MsgType = "text"
	req.AgentID = cfg.AgentID
	req.Text.Content = content

	data, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Error("wecom send request failed", "err", err)
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("wecom send read body failed", "err", err)
		return "", err
	}

	var sr apiResp
	if err := json.Unmarshal(body, &sr); err != nil {
		log.Error("wecom send parse failed", "err", err, "body", string(body))
		return "", fmt.Errorf("解析发送响应失败: %w, body=%s", err, body)
	}

	if sr.ErrCode != 0 {
		log.Error("wecom send api error", "errcode", sr.ErrCode, "errmsg", sr.ErrMsg)
		return "", fmt.Errorf("发送失败: errcode=%d errmsg=%s", sr.ErrCode, sr.ErrMsg)
	}

	log.Info("wecom send ok", "msgid", sr.MsgID)
	return sr.MsgID, nil
}

// SendTest 向指定成员发送默认测试文案。toUser 为空时使用 WECOM_TO_USER。
func SendTest(toUser string) (msgID string, err error) {
	cfg, err := LoadConfig()
	if err != nil {
		logger.Default().Error("wecom load config failed", "err", err)
		return "", err
	}

	content := "📢 项目提醒测试（来自项目看板）\n时间：" + time.Now().Format("2006-01-02 15:04:05")
	return SendText(cfg, toUser, content)
}
