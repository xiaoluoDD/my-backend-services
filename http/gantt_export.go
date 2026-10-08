package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/xiaoluoDD/my-backend-services/internal/ganttexport"
)

func handleGanttExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 POST",
		})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "读取请求失败",
		})
		return
	}
	var req ganttexport.Request
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请求体格式错误",
		})
		return
	}
	data, err := ganttexport.Build(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	name := strings.TrimSpace(req.WorkNo)
	if name == "" {
		name = "gantt"
	}
	name = strings.Map(func(r rune) rune {
		switch r {
		case '\\', '/', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		default:
			return r
		}
	}, name)
	if !utf8.ValidString(name) {
		name = "gantt"
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`_gantt.xlsx"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
