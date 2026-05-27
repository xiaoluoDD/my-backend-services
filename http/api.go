package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/xiaoluoDD/my-backend-services/internal/wecom"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	http.HandleFunc("/say", func(w http.ResponseWriter, r *http.Request) {
		msg := r.URL.Query().Get("msg")
		if msg == "" {
			msg = "(empty)"
		}
		log.Printf("/say msg=%q\n", msg)

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

		log.Printf("/api/wecom/test from %s\n", r.RemoteAddr)

		msgID, err := wecom.SendTest()
		if err != nil {
			log.Printf("wecom send failed: %v\n", err)
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"ok":    false,
				"error": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":         true,
			"msg":        "企业微信测试消息已发送",
			"msgid":      msgID,
			"server_now": time.Now().Format("2006-01-02 15:04:05"),
		})
	})

	log.Println("HTTP API listening on :8080")
	log.Println("  GET/POST http://<host>:8080/api/wecom/test  — 发送企业微信测试")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
