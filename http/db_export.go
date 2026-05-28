package main

import (
	"net/http"
	"time"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
)

func handleDBExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 GET",
		})
		return
	}

	users, err := db.ListActiveUsers(sqlDB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	projects, err := db.ListProjects(sqlDB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	stats, err := db.Stats(sqlDB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"exported_at": now,
		"server_now":  now,
		"count": map[string]int{
			"users":    len(users),
			"projects": len(projects),
		},
		"users":    users,
		"projects": projects,
		"stats":    stats,
	})
}
