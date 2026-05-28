package main

import (
	"os"

	"github.com/xiaoluoDD/my-backend-services/internal/logger"
	"github.com/xiaoluoDD/my-backend-services/internal/wecom"
)

func main() {
	log := logger.Init("wecom")
	log.Info("cli test send starting")

	msgID, err := wecom.SendTest()
	if err != nil {
		log.Error("cli test send failed", "err", err)
		os.Exit(1)
	}

	log.Info("cli test send ok", "msgid", msgID)
}