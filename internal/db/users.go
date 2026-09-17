package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// AppUser 应用可见范围内的成员（持久化）。
// 一个成员可以同时属于多个部门（对应企业微信里一人多部门的情况）：
//   - DepartmentIDs / DepartmentList：该成员当前所属的全部部门（多对多关系表 app_user_departments）。
//   - DepartmentID / DepartmentName：兼容字段，取 DepartmentIDs 的第一个作为"主部门"，
//     供历史上按单部门实现的功能（如项目负责人默认部门定位）使用；新代码请优先使用 DepartmentIDs。
type AppUser struct {
	UserID         string          `json:"userid"`
	Name           string          `json:"name"`
	Mobile         string          `json:"mobile,omitempty"`
	Departments    string          `json:"departments"`
	DepartmentID   int64           `json:"department_id,omitempty"`
	DepartmentName string          `json:"department_name,omitempty"`
	DepartmentIDs  []int64         `json:"department_ids,omitempty"`
	DepartmentList []DepartmentRef `json:"department_list,omitempty"`
	Sources        string          `json:"sources"`
	UpdatedAt      string          `json:"updated_at"`
}

// DepartmentRef 成员所属部门的简要引用。
type DepartmentRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// SyncRun 一次同步任务记录。
type SyncRun struct {
	ID           int64  `json:"id"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at,omitempty"`
	Status       string `json:"status"`
	UserCount    int    `json:"user_count"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// UpsertCorpInfo 写入企业/应用配置快照。
func UpsertCorpInfo(db *sql.DB, pairs map[string]string) error {
	now := time.Now().Format(time.RFC3339)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for k, v := range pairs {
		_, err := tx.Exec(
			`INSERT INTO corp_info (key, value, updated_at) VALUES (?, ?, ?)
			 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
			k, v, now,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListCorpInfo 读取企业配置快照。
func ListCorpInfo(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query(`SELECT key, value FROM corp_info ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// BeginSync 创建同步任务记录，返回 run ID。
func BeginSync(db *sql.DB) (int64, error) {
	now := time.Now().Format(time.RFC3339)
	res, err := db.Exec(
		`INSERT INTO sync_runs (started_at, status) VALUES (?, 'running')`,
		now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// FinishSync 结束同步任务。
func FinishSync(db *sql.DB, runID int64, status string, userCount int, errMsg string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(
		`UPDATE sync_runs SET finished_at=?, status=?, user_count=?, error_message=? WHERE id=?`,
		now, status, userCount, errMsg, runID,
	)
	return err
}

// LastSyncRun 返回最近一次同步记录。
func LastSyncRun(db *sql.DB) (*SyncRun, error) {
	row := db.QueryRow(
		`SELECT id, started_at, finished_at, status, user_count, error_message
		 FROM sync_runs ORDER BY id DESC LIMIT 1`,
	)
	var s SyncRun
	if err := row.Scan(&s.ID, &s.StartedAt, &s.FinishedAt, &s.Status, &s.UserCount, &s.ErrorMessage); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// ReplaceAppUsers 全量替换当前可见成员（事务）。
func ReplaceAppUsers(db *sql.DB, users []AppUser) error {
	now := time.Now().Format(time.RFC3339)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE app_users SET active=0`); err != nil {
		return err
	}

	// 部门以企业微信通讯录同步结果为权威来源：
	// - 本次同步在企业微信里找到部门（len(DepartmentIDs) > 0）时，直接覆盖 app_user_departments
	//   （多对多关系表，支持一人多部门），并把第一个部门写作兼容用的 department_id/departments。
	// - 否则（本次未能取到部门，例如接口失败或该成员不在任何可见部门）保留原有归属，
	//   避免把已有的部门数据清空。
	stmt, err := tx.Prepare(
		`INSERT INTO app_users (userid, name, mobile, departments, department_id, sources, active, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, 1, ?)
		 ON CONFLICT(userid) DO UPDATE SET
		   name=excluded.name,
		   mobile=CASE WHEN excluded.mobile != '' THEN excluded.mobile ELSE app_users.mobile END,
		   departments=excluded.departments,
		   department_id=CASE WHEN excluded.department_id > 0 THEN excluded.department_id ELSE app_users.department_id END,
		   sources=excluded.sources,
		   active=1,
		   updated_at=excluded.updated_at`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	delDeptStmt, err := tx.Prepare(`DELETE FROM app_user_departments WHERE userid=?`)
	if err != nil {
		return err
	}
	defer delDeptStmt.Close()

	insDeptStmt, err := tx.Prepare(
		`INSERT OR IGNORE INTO app_user_departments (userid, department_id) VALUES (?, ?)`,
	)
	if err != nil {
		return err
	}
	defer insDeptStmt.Close()

	for _, u := range users {
		updated := u.UpdatedAt
		if updated == "" {
			updated = now
		}
		var primaryDeptID int64
		if len(u.DepartmentIDs) > 0 {
			primaryDeptID = u.DepartmentIDs[0]
		}
		if _, err := stmt.Exec(u.UserID, u.Name, u.Mobile, u.Departments, primaryDeptID, u.Sources, updated); err != nil {
			return err
		}
		if len(u.DepartmentIDs) == 0 {
			// 本次同步未取得该成员的部门数据，保留原有的多部门归属不动。
			continue
		}
		if _, err := delDeptStmt.Exec(u.UserID); err != nil {
			return err
		}
		for _, deptID := range u.DepartmentIDs {
			if deptID <= 0 {
				continue
			}
			if _, err := insDeptStmt.Exec(u.UserID, deptID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// ListActiveUsers 列出当前有效成员（含各自的多部门归属）。
func ListActiveUsers(db *sql.DB) ([]AppUser, error) {
	rows, err := db.Query(
		`SELECT u.userid, u.name, u.mobile, u.departments, u.department_id,
		        COALESCE(d.name, ''), u.sources, u.updated_at
		 FROM app_users u
		 LEFT JOIN departments d ON u.department_id = d.id
		 WHERE u.active=1
		 ORDER BY u.name, u.userid`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users, err := scanAppUsers(rows)
	if err != nil {
		return nil, err
	}
	if err := attachDepartments(db, users); err != nil {
		return nil, err
	}
	return users, nil
}

// attachDepartments 批量填充成员的完整部门列表（DepartmentIDs / DepartmentList），
// 并把 DepartmentName 重写为「所有所属部门名称」的拼接（多个用「、」分隔），
// 供成员管理等界面展示一人多部门；DepartmentID/DepartmentName 兼容字段则取第一个部门。
func attachDepartments(sqlDB *sql.DB, users []AppUser) error {
	if len(users) == 0 {
		return nil
	}
	idx := make(map[string]int, len(users))
	placeholders := make([]string, 0, len(users))
	args := make([]interface{}, 0, len(users))
	for i, u := range users {
		if u.UserID == "" {
			continue
		}
		idx[u.UserID] = i
		placeholders = append(placeholders, "?")
		args = append(args, u.UserID)
	}
	if len(placeholders) == 0 {
		return nil
	}

	query := `SELECT ud.userid, d.id, d.name
	          FROM app_user_departments ud
	          INNER JOIN departments d ON d.id = ud.department_id
	          WHERE ud.userid IN (` + strings.Join(placeholders, ",") + `)
	          ORDER BY ud.userid, d.name`
	rows, err := sqlDB.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var userid string
		var ref DepartmentRef
		if err := rows.Scan(&userid, &ref.ID, &ref.Name); err != nil {
			return err
		}
		i, ok := idx[userid]
		if !ok {
			continue
		}
		users[i].DepartmentList = append(users[i].DepartmentList, ref)
		users[i].DepartmentIDs = append(users[i].DepartmentIDs, ref.ID)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range users {
		if len(users[i].DepartmentList) == 0 {
			continue
		}
		names := make([]string, len(users[i].DepartmentList))
		for j, d := range users[i].DepartmentList {
			names[j] = d.Name
		}
		users[i].DepartmentID = users[i].DepartmentList[0].ID
		users[i].DepartmentName = strings.Join(names, "、")
	}
	return nil
}

func scanAppUsers(rows *sql.Rows) ([]AppUser, error) {
	var list []AppUser
	for rows.Next() {
		var u AppUser
		if err := rows.Scan(
			&u.UserID, &u.Name, &u.Mobile, &u.Departments, &u.DepartmentID,
			&u.DepartmentName, &u.Sources, &u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, u)
	}
	return list, rows.Err()
}

// GetAppUser 按 userid 查询成员（含多部门归属）。
func GetAppUser(db *sql.DB, userid string) (AppUser, error) {
	row := db.QueryRow(
		`SELECT u.userid, u.name, u.mobile, u.departments, u.department_id,
		        COALESCE(d.name, ''), u.sources, u.updated_at
		 FROM app_users u
		 LEFT JOIN departments d ON u.department_id = d.id
		 WHERE u.userid=? AND u.active=1`,
		userid,
	)
	var u AppUser
	if err := row.Scan(
		&u.UserID, &u.Name, &u.Mobile, &u.Departments, &u.DepartmentID,
		&u.DepartmentName, &u.Sources, &u.UpdatedAt,
	); err != nil {
		return AppUser{}, err
	}
	users := []AppUser{u}
	if err := attachDepartments(db, users); err != nil {
		return AppUser{}, err
	}
	return users[0], nil
}

// UpdateAppUser 更新成员手机号与部门归属（手动维护字段，支持一人多部门）。
// departmentIDs 为该成员应归属的完整部门列表（全量替换）；传空切片表示清空所有部门归属。
func UpdateAppUser(db *sql.DB, userid, mobile string, departmentIDs []int64) (AppUser, error) {
	if userid == "" {
		return AppUser{}, fmt.Errorf("userid 不能为空")
	}

	// 去重并校验部门是否存在。
	seen := make(map[int64]struct{}, len(departmentIDs))
	cleanIDs := make([]int64, 0, len(departmentIDs))
	var primaryName string
	for i, id := range departmentIDs {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		d, err := GetDepartment(db, id)
		if err != nil {
			return AppUser{}, fmt.Errorf("部门不存在")
		}
		if i == 0 || primaryName == "" {
			primaryName = d.Name
		}
		cleanIDs = append(cleanIDs, id)
	}

	tx, err := db.Begin()
	if err != nil {
		return AppUser{}, err
	}
	defer tx.Rollback()

	now := time.Now().Format(time.RFC3339)
	var primaryID int64
	if len(cleanIDs) > 0 {
		primaryID = cleanIDs[0]
	}
	res, err := tx.Exec(
		`UPDATE app_users SET mobile=?, department_id=?, departments=?, updated_at=?
		 WHERE userid=? AND active=1`,
		mobile, primaryID, primaryName, now, userid,
	)
	if err != nil {
		return AppUser{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return AppUser{}, err
	}
	if n == 0 {
		return AppUser{}, sql.ErrNoRows
	}

	if _, err := tx.Exec(`DELETE FROM app_user_departments WHERE userid=?`, userid); err != nil {
		return AppUser{}, err
	}
	for _, id := range cleanIDs {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO app_user_departments (userid, department_id) VALUES (?, ?)`,
			userid, id,
		); err != nil {
			return AppUser{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return AppUser{}, err
	}
	return GetAppUser(db, userid)
}

// CountActiveUsers 统计有效成员数。
func CountActiveUsers(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM app_users WHERE active=1`).Scan(&n)
	return n, err
}

// Stats 汇总数据库状态。
func Stats(db *sql.DB) (map[string]interface{}, error) {
	count, err := CountActiveUsers(db)
	if err != nil {
		return nil, err
	}
	last, err := LastSyncRun(db)
	if err != nil {
		return nil, err
	}
	corp, err := ListCorpInfo(db)
	if err != nil {
		return nil, err
	}

	out := map[string]interface{}{
		"active_users": count,
		"corp_info":    corp,
	}
	if last != nil {
		out["last_sync"] = last
	}
	return out, nil
}

// FormatSyncError 包装同步错误信息。
func FormatSyncError(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%v", err)
}
