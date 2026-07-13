package main

import (
	"net/http"

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
