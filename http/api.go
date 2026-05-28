package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/xiaoluoDD/my-backend-services/internal/logger"
	"github.com/xiaoluoDD/my-backend-services/internal/wecom"
)

func listenAddr() string {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8081"
	}
	return ":" + port
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	appLog := logger.Init("http")
	appLog.Info("starting http api", "addr", listenAddr(), "log_dir", os.Getenv("LOG_DIR"))

	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	http.HandleFunc("/say", func(w http.ResponseWriter, r *http.Request) {
		msg := r.URL.Query().Get("msg")
		if msg == "" {
			msg = "(empty)"
		}
		appLog.Debug("say", "msg", msg)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":         true,
			"you_sent":   msg,
			"server_now": time.Now().Format("2006-01-02 15:04:05"),
		})
	})

	// 项目看板 Qt 客户端调用：触发一次企业微信测试消息
	http.HandleFunc("/api/wecom/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
				"ok":    false,
				"error": "请使用 GET 或 POST",
			})
			return
		}

		appLog.Info("wecom test requested", "remote", r.RemoteAddr)

		msgID, err := wecom.SendTest()
		if err != nil {
			appLog.Error("wecom send failed", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"ok":    false,
				"error": err.Error(),
			})
			return
		}

		appLog.Info("wecom test sent", "msgid", msgID)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":         true,
			"msg":        "企业微信测试消息已发送",
			"msgid":      msgID,
			"server_now": time.Now().Format("2006-01-02 15:04:05"),
		})
	})

	addr := listenAddr()
	appLog.Info("routes ready",
		"ping", "GET /ping",
		"wecom_test", "GET|POST /api/wecom/test",
	)
	log.Fatal(http.ListenAndServe(addr, logger.HTTPMiddleware(http.DefaultServeMux)))
}
