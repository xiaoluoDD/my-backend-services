package main

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/xiaoluoDD/my-backend-services/internal/logger"
)

func handleLogs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleLogList(w, r)
	case http.MethodDelete:
		handleLogDelete(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 GET / DELETE",
		})
	}
}

func handleLogList(w http.ResponseWriter, r *http.Request) {
	files, err := logger.ListLogFiles()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":    true,
		"dir":   logger.LogDir(),
		"count": len(files),
		"files": files,
	})
}

func handleLogDelete(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "请提供 ?name=日志文件名",
		})
		return
	}

	if err := logger.DeleteLogFile(name); err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusNotFound, map[string]interface{}{
				"ok": false, "error": "日志文件不存在",
			})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":  true,
		"msg": "日志已删除",
		"name": filepath.Base(name),
	})
}

func handleLogDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok": false, "error": "请使用 GET",
		})
		return
	}

	name := r.URL.Query().Get("name")
	path, err := logger.ResolveLogFile(name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(name))
	http.ServeFile(w, r, path)
}
