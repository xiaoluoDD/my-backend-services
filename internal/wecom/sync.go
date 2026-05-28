package wecom

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
	"github.com/xiaoluoDD/my-backend-services/internal/logger"
)

// SyncResult 同步结果。
type SyncResult struct {
	RunID     int64  `json:"run_id"`
	UserCount int    `json:"user_count"`
	Status    string `json:"status"`
}

// SyncUsersToDB 从企业微信拉取应用可见成员并写入 SQLite。
func SyncUsersToDB(sqlDB *sql.DB) (*SyncResult, error) {
	log := logger.Default()

	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	runID, err := db.BeginSync(sqlDB)
	if err != nil {
		return nil, fmt.Errorf("begin sync: %w", err)
	}

	finish := func(status string, count int, syncErr error) (*SyncResult, error) {
		errMsg := db.FormatSyncError(syncErr)
		if err := db.FinishSync(sqlDB, runID, status, count, errMsg); err != nil {
			log.Error("finish sync record failed", "err", err)
		}
		if syncErr != nil {
			return &SyncResult{RunID: runID, UserCount: count, Status: status}, syncErr
		}
		return &SyncResult{RunID: runID, UserCount: count, Status: status}, nil
	}

	log.Info("wecom sync started", "run_id", runID)

	token, err := getToken(cfg)
	if err != nil {
		return finish("failed", 0, err)
	}

	scope, err := FetchAgentScope(cfg, token)
	if err != nil {
		return finish("failed", 0, err)
	}

	members := make(map[string]*member)

	addUser := func(userid, name, mobile string, depts []int, source string) {
		if userid == "" {
			return
		}
		m, ok := members[userid]
		if !ok {
			m = &member{UserID: userid, Departments: depts}
			members[userid] = m
		}
		if name != "" {
			m.Name = name
		}
		if mobile != "" {
			m.Mobile = mobile
		}
		if len(depts) > 0 {
			m.Departments = depts
		}
		m.addSource(source)
	}

	for _, u := range scope.AllowUserInfos.User {
		addUser(u.UserID, u.Name, "", nil, "allow_user")
	}

	for _, partyID := range scope.AllowParties.PartyID {
		ur, err := FetchDepartmentUsers(token, partyID)
		if err != nil {
			return finish("failed", 0, err)
		}
		for _, u := range ur.UserList {
			addUser(u.UserID, u.Name, u.Mobile, u.Department, fmt.Sprintf("party:%d", partyID))
		}
	}

	for _, tagID := range scope.AllowTags.TagID {
		tr, err := FetchTagMembers(token, tagID)
		if err != nil {
			return finish("failed", 0, err)
		}
		for _, u := range tr.UserList {
			addUser(u.UserID, u.Name, "", nil, fmt.Sprintf("tag:%d", tagID))
		}
		for _, p := range tr.PartyList {
			ur, err := FetchDepartmentUsers(token, p.PartyID)
			if err != nil {
				return finish("failed", 0, err)
			}
			for _, u := range ur.UserList {
				addUser(u.UserID, u.Name, u.Mobile, u.Department, fmt.Sprintf("tag:%d/party:%d", tagID, p.PartyID))
			}
		}
	}

	now := time.Now().Format(time.RFC3339)
	users := make([]db.AppUser, 0, len(members))
	for _, m := range members {
		users = append(users, db.AppUser{
			UserID:      m.UserID,
			Name:        m.Name,
			Mobile:      m.Mobile,
			Departments: joinDepartments(m.Departments),
			Sources:     joinSources(m.Sources),
			UpdatedAt:   now,
		})
	}

	if err := db.ReplaceAppUsers(sqlDB, users); err != nil {
		return finish("failed", 0, fmt.Errorf("save users: %w", err))
	}

	if err := db.UpsertCorpInfo(sqlDB, map[string]string{
		"corp_id":  cfg.CorpID,
		"agent_id": fmt.Sprintf("%d", cfg.AgentID),
		"to_user":  cfg.ToUser,
	}); err != nil {
		return finish("failed", len(users), fmt.Errorf("save corp info: %w", err))
	}

	log.Info("wecom sync ok", "run_id", runID, "user_count", len(users))
	return finish("ok", len(users), nil)
}
