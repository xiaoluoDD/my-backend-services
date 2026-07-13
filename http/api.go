package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
	"github.com/xiaoluoDD/my-backend-services/internal/logger"
	"github.com/xiaoluoDD/my-backend-services/internal/reminder"
	"github.com/xiaoluoDD/my-backend-services/internal/wecom"
)

var sqlDB *sql.DB

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

	var err error
	sqlDB, err = db.Open("")
	if err != nil {
		appLog.Error("open database failed", "err", err)
		log.Fatal(err)
	}
	defer sqlDB.Close()
	appLog.Info("database ready", "path", db.FilePath())

	reminder.StartScheduler(sqlDB)

	appLog.Info("starting http api", "addr", listenAddr())

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

	http.HandleFunc("/api/wecom/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
				"ok": false, "error": "请使用 GET 或 POST",
			})
			return
		}

		userid := r.URL.Query().Get("userid")
		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			if len(body) > 0 {
				var req struct {
					UserID string `json:"userid"`
					Name   string `json:"name"`
				}
				if err := json.Unmarshal(body, &req); err == nil && req.UserID != "" {
					userid = req.UserID
				}
			}
		}

		if userid == "" {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"ok":    false,
				"error": "请指定 userid（JSON: {\"userid\":\"xxx\"} 或查询参数 ?userid=xxx）",
			})
			return
		}

		appLog.Info("wecom test requested", "remote", r.RemoteAddr, "userid", userid)

		msgID, err := wecom.SendTest(userid)
		if err != nil {
			appLog.Error("wecom send failed", "err", err, "userid", userid)
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"ok": false, "error": err.Error(),
			})
			return
		}

		appLog.Info("wecom test sent", "msgid", msgID, "userid", userid)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":         true,
			"msg":        "企业微信测试消息已发送",
			"msgid":      msgID,
			"to_user":    userid,
			"server_now": time.Now().Format("2006-01-02 15:04:05"),
		})
	})

	// 从企业微信同步应用可见成员到 SQLite
	http.HandleFunc("/api/wecom/sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
				"ok": false, "error": "请使用 GET 或 POST",
			})
			return
		}

		appLog.Info("wecom sync requested", "remote", r.RemoteAddr)
		result, err := wecom.SyncUsersToDB(sqlDB)
		if err != nil {
			appLog.Error("wecom sync failed", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"ok":    false,
				"error": err.Error(),
				"sync":  result,
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":         true,
			"msg":        "成员同步完成",
			"sync":       result,
			"server_now": time.Now().Format("2006-01-02 15:04:05"),
		})
	})

	// 查询已持久化的成员列表 / 更新成员信息
	http.HandleFunc("/api/wecom/users", handleWecomUsers)

	// 数据库与企业信息概览
	http.HandleFunc("/api/wecom/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
				"ok": false, "error": "请使用 GET",
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
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":    true,
			"stats": stats,
		})
	})

	http.HandleFunc("/api/settings", handleSettings)
	http.HandleFunc("/api/projects", handleProjects)
	http.HandleFunc("/api/project-subtasks", handleProjectSubtasks)
	http.HandleFunc("/api/dashboard/summary", handleDashboardSummary)
	http.HandleFunc("/api/dashboard/person-tasks", handleDashboardPersonTasks)
	http.HandleFunc("/api/changelog", handleChangelog)
	http.HandleFunc("/api/departments", handleDepartments)
	http.HandleFunc("/api/db/export", handleDBExport)
	http.HandleFunc("/api/db/download", handleDBDownload)
	http.HandleFunc("/api/wecom/send-group", handleWecomSendGroup)
	http.HandleFunc("/api/wecom/notify-project", handleWecomNotifyProject)
	http.HandleFunc("/api/reminder/run", handleReminderRun)

	http.HandleFunc("/api/logs", handleLogs)
	http.HandleFunc("/api/logs/download", handleLogDownload)

	addr := listenAddr()
	appLog.Info("routes ready",
		"ping", "GET /ping",
		"wecom_test", "GET|POST /api/wecom/test",
		"wecom_sync", "GET|POST /api/wecom/sync",
		"wecom_users", "GET|PUT /api/wecom/users",
		"departments", "GET|POST|PUT|DELETE /api/departments",
		"wecom_stats", "GET /api/wecom/stats",
		"settings", "GET|PUT /api/settings",
		"projects", "GET|POST|PUT|DELETE /api/projects",
		"project_subtasks", "GET|POST|PUT|DELETE /api/project-subtasks",
		"dashboard_summary", "GET /api/dashboard/summary?year=",
		"dashboard_person_tasks", "GET /api/dashboard/person-tasks?userid=&name=&status=&year=",
		"changelog", "GET /api/changelog",
		"db_export", "GET /api/db/export",
		"db_download", "GET /api/db/download",
		"wecom_send_group", "POST /api/wecom/send-group",
		"wecom_notify_project", "POST /api/wecom/notify-project",
		"reminder_run", "GET|POST /api/reminder/run",
		"logs", "GET|DELETE /api/logs",
		"logs_download", "GET /api/logs/download?name=",
	)
	log.Fatal(http.ListenAndServe(addr, logger.HTTPMiddleware(http.DefaultServeMux)))
}
