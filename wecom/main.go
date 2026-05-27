package main

import (
	"fmt"
	"os"

	"github.com/xiaoluoDD/my-backend-services/internal/wecom"
)

func main() {
	fmt.Println("🔑 获取 access_token 并发送测试消息...")
	msgID, err := wecom.SendTest()
	if err != nil {
		fmt.Println("❌", err)
		os.Exit(1)
	}

	fmt.Println("🎉 消息已发送！msgid:", msgID)
	fmt.Println("请检查企业微信")
}
