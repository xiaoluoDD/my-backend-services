package main

import (
	"net/http"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
	"github.com/xiaoluoDD/my-backend-services/internal/reminder"
)

func handleReminderRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 GET 或 POST",
		})
		return
	}

	settings, err := db.GetAppSettings(sqlDB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	result := reminder.RunDaily(sqlDB, settings)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":     true,
		"msg":    "提醒扫描已完成",
		"result": result,
	})
}
