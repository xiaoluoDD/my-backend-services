package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

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

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":         true,
			"you_sent":   msg,
			"server_now": time.Now().Format("2006-01-02 15:04:05"),
		})
	})

	log.Println("HTTP API listening on :8080")
	http.ListenAndServe(":8080", nil)
}
