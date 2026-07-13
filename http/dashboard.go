package main

import (
	"net/http"
	"strings"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
)

func handleDashboardSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 GET",
		})
		return
	}

	year := r.URL.Query().Get("year")
	summary, err := db.SummarizeDashboard(sqlDB, year)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"summary": summary,
	})
}

func handleDashboardPersonTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 GET",
		})
		return
	}

	userid := r.URL.Query().Get("userid")
	name := r.URL.Query().Get("name")
	if strings.TrimSpace(userid) == "" && strings.TrimSpace(name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请指定 userid 或 name",
		})
		return
	}

	status := r.URL.Query().Get("status")
	year := r.URL.Query().Get("year")
	rows, err := db.ListDashboardPersonTasks(sqlDB, userid, name, status, year)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":    true,
		"tasks": rows,
	})
}
