package main

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

type config struct {
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

func loadConfig() (config, error) {
	corpID := os.Getenv("WECOM_CORP_ID")
	secret := os.Getenv("WECOM_SECRET")
	toUser := os.Getenv("WECOM_TO_USER")
	agentStr := os.Getenv("WECOM_AGENT_ID")

	if corpID == "" || secret == "" || toUser == "" || agentStr == "" {
		return config{}, fmt.Errorf(
			"缺少环境变量，请设置 WECOM_CORP_ID、WECOM_SECRET、WECOM_AGENT_ID、WECOM_TO_USER（可参考 .env.example）",
		)
	}

	agentID, err := strconv.Atoi(agentStr)
	if err != nil {
		return config{}, fmt.Errorf("WECOM_AGENT_ID 必须是数字: %w", err)
	}

	return config{
		CorpID:  corpID,
		AgentID: agentID,
		Secret:  secret,
		ToUser:  toUser,
	}, nil
}

func getToken(cfg config) (string, error) {
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
		return "", fmt.Errorf("解析 token 响应失败: %w, body=%s", err, body)
	}

	if tr.ErrCode != 0 {
		return "", fmt.Errorf("gettoken 失败: errcode=%d errmsg=%s", tr.ErrCode, tr.ErrMsg)
	}

	return tr.AccessToken, nil
}

func sendText(cfg config, token, content string) error {
	url := "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=" + token

	var req sendReq
	req.ToUser = cfg.ToUser
	req.MsgType = "text"
	req.AgentID = cfg.AgentID
	req.Text.Content = content

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

	fmt.Println("📨 企业微信返回：", string(body))

	var sr apiResp
	if err := json.Unmarshal(body, &sr); err != nil {
		return fmt.Errorf("解析发送响应失败: %w", err)
	}

	if sr.ErrCode != 0 {
		return fmt.Errorf("发送失败: errcode=%d errmsg=%s", sr.ErrCode, sr.ErrMsg)
	}

	return nil
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("❌ 配置错误：", err)
		os.Exit(1)
	}

	fmt.Println("🔑 获取 access_token...")
	token, err := getToken(cfg)
	if err != nil {
		fmt.Println("❌", err)
		os.Exit(1)
	}

	fmt.Println("✅ token 获取成功，发送消息...")
	content := "📢 项目提醒测试\n时间：" + time.Now().Format("2006-01-02 15:04:05")
	if err := sendText(cfg, token, content); err != nil {
		fmt.Println("❌", err)
		os.Exit(1)
	}

	fmt.Println("🎉 消息已发送！请检查企业微信")
}
