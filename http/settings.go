package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
)

type settingsPayload struct {
	ServerBaseURL string `json:"server_base_url"`
}

func normalizeBaseURL(raw string) string {
	s := strings.TrimSpace(raw)
	for strings.HasSuffix(s, "/") {
		s = strings.TrimSuffix(s, "/")
	}
	return s
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getSettings(w, r)
	case http.MethodPut:
		putSettings(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 GET 或 PUT",
		})
	}
}

func getSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := db.GetAppSettings(sqlDB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"settings": settings,
	})
}

func putSettings(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "读取请求体失败",
		})
		return
	}

	var p settingsPayload
	if len(body) > 0 {
		if err := json.Unmarshal(body, &p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"ok": false, "error": "请求体格式错误",
			})
			return
		}
	}

	p.ServerBaseURL = normalizeBaseURL(p.ServerBaseURL)
	if p.ServerBaseURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "server_base_url 不能为空",
		})
		return
	}

	if err := db.SaveAppSettings(sqlDB, db.AppSettings{ServerBaseURL: p.ServerBaseURL}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	saved, _ := db.GetAppSettings(sqlDB)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "msg": "设置已保存", "settings": saved,
	})
}
