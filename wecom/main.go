package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ===== 改成你自己的 =====
const (
	CorpID  = "ww411e84e02e63af6d"      // 企业ID
	AgentID = 1000005               // 应用 AgentID（数字）
	Secret  = "6OQQHiqQdEpC0MuUoxl_JUEKY3lPC0tYFMVLMzO7BMQ" // 应用 Secret
	ToUser  = "LuoXian"            // 员工 UserID（账号）
)

// 获取 access_token 的响应结构
type TokenResp struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// 发送消息的请求结构
type SendReq struct {
	ToUser  string `json:"touser"`
	MsgType string `json:"msgtype"`
	AgentID int    `json:"agentid"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

func getToken() (string, error) {
	url := fmt.Sprintf(
		"https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		CorpID, Secret,
	)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var tr TokenResp
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", err
	}

	if tr.ErrCode != 0 {
		return "", fmt.Errorf("get token failed: %d %s", tr.ErrCode, tr.ErrMsg)
	}

	return tr.AccessToken, nil
}

func sendText(token, content string) error {
	url := "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=" + token

	var req SendReq
	req.ToUser = ToUser
	req.MsgType = "text"
	req.AgentID = AgentID
	req.Text.Content = content

	data, _ := json.Marshal(req)

	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println("📨 企业微信返回：", string(body))
	return nil
}

func main() {
	fmt.Println("🔑 获取 access_token...")
	token, err := getToken()
	if err != nil {
		fmt.Println("❌", err)
		return
	}

	fmt.Println("✅ token 获取成功，发送消息...")
	err = sendText(token, "📢 项目提醒测试\n时间："+time.Now().Format("2006-01-02 15:04:05"))
	if err != nil {
		fmt.Println("❌ 发送失败：", err)
		return
	}

	fmt.Println("🎉 消息已发送！请检查企业微信")
}
