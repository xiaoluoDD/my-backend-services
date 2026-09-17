package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Department 部门。既可以是手动创建的部门（WecomDeptID=0），
// 也可以是从企业微信通讯录同步导入的部门（WecomDeptID>0，名称以企业微信为准）。
type Department struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	WecomDeptID int64  `json:"wecom_dept_id"`
	UpdatedAt   string `json:"updated_at"`
}

// IsFromWecom 是否为企业微信同步导入的部门。
func (d Department) IsFromWecom() bool {
	return d.WecomDeptID > 0
}

// DepartmentView 部门及其成员列表。
type DepartmentView struct {
	Department
	MemberCount int       `json:"member_count"`
	Members     []AppUser `json:"members,omitempty"`
}

// ListDepartments 返回全部部门（按名称排序）。
func ListDepartments(db *sql.DB) ([]Department, error) {
	rows, err := db.Query(
		`SELECT id, name, wecom_dept_id, updated_at FROM departments ORDER BY name, id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Department
	for rows.Next() {
		var d Department
		if err := rows.Scan(&d.ID, &d.Name, &d.WecomDeptID, &d.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	return list, rows.Err()
}

// ListDepartmentViews 返回部门列表，可选附带成员。
func ListDepartmentViews(db *sql.DB, withMembers bool) ([]DepartmentView, error) {
	depts, err := ListDepartments(db)
	if err != nil {
		return nil, err
	}

	views := make([]DepartmentView, 0, len(depts))
	for _, d := range depts {
		members, err := ListUsersByDepartmentID(db, d.ID)
		if err != nil {
			return nil, err
		}
		views = append(views, DepartmentView{
			Department:  d,
			MemberCount: len(members),
			Members:     membersIf(withMembers, members),
		})
	}
	return views, nil
}

func membersIf(include bool, members []AppUser) []AppUser {
	if !include {
		return nil
	}
	if members == nil {
		return []AppUser{}
	}
	return members
}

// GetDepartment 按 ID 查询部门。
func GetDepartment(db *sql.DB, id int64) (Department, error) {
	var d Department
	err := db.QueryRow(
		`SELECT id, name, wecom_dept_id, updated_at FROM departments WHERE id=?`, id,
	).Scan(&d.ID, &d.Name, &d.WecomDeptID, &d.UpdatedAt)
	return d, err
}

// CreateDepartment 已停用：部门统一由企业微信通讯录同步生成（见 UpsertWecomDepartments），
// 不再支持手动新增，避免产生与企业微信组织架构无关、导致归属混乱的部门。
// 如需新增部门，请在企业微信管理后台创建后，点击「同步成员」即可自动导入。
func CreateDepartment(db *sql.DB, name string) (int64, error) {
	return 0, fmt.Errorf("部门已改为自动跟随企业微信通讯录，不支持手动新增；请在企业微信管理后台创建部门后重新同步成员")
}

// UpdateDepartment 更新部门名称（仅限手动创建的部门；企业微信同步导入的部门名称
// 以企业微信通讯录为准，会在下次同步时被覆盖，因此禁止在本地手动修改）。
func UpdateDepartment(db *sql.DB, id int64, name string) error {
	if id <= 0 {
		return fmt.Errorf("无效的部门 ID")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("部门名称不能为空")
	}
	existing, err := GetDepartment(db, id)
	if err != nil {
		return err
	}
	if existing.IsFromWecom() {
		return fmt.Errorf("该部门来自企业微信通讯录同步，名称请在企业微信管理后台修改，同步后会自动更新")
	}
	now := time.Now().Format(time.RFC3339)
	res, err := db.Exec(
		`UPDATE departments SET name=?, updated_at=? WHERE id=?`, name, now, id,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}

	_, err = db.Exec(
		`UPDATE app_users SET departments=? WHERE department_id=?`, name, id,
	)
	return err
}

// DeleteDepartment 删除部门。该部门下成员与部门的多对多关系（app_user_departments）
// 会通过外键级联自动清除；随后重新计算受影响成员的兼容字段（主部门 department_id/departments，
// 取其剩余部门中的第一个，若已无任何部门归属则清零）。
func DeleteDepartment(db *sql.DB, id int64) error {
	if id <= 0 {
		return fmt.Errorf("无效的部门 ID")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	affected, err := queryUserIDs(tx, `SELECT userid FROM app_user_departments WHERE department_id=?`, id)
	if err != nil {
		return err
	}

	res, err := tx.Exec(`DELETE FROM departments WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}

	now := time.Now().Format(time.RFC3339)
	for _, userid := range affected {
		var newPrimaryID sql.NullInt64
		var newPrimaryName sql.NullString
		row := tx.QueryRow(
			`SELECT d.id, d.name FROM app_user_departments ud
			 INNER JOIN departments d ON d.id = ud.department_id
			 WHERE ud.userid=? ORDER BY d.name LIMIT 1`,
			userid,
		)
		_ = row.Scan(&newPrimaryID, &newPrimaryName) // 无剩余部门时保持零值，即清零

		if _, err := tx.Exec(
			`UPDATE app_users SET department_id=?, departments=?, updated_at=? WHERE userid=?`,
			newPrimaryID.Int64, newPrimaryName.String, now, userid,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteManualDepartments 批量删除所有"纯手动创建"（wecom_dept_id=0）的部门，
// 用于一次性清理改用企业微信同步部门之前遗留下来的旧手动部门。
// 逐个走 DeleteDepartment，复用其级联清理与兼容字段重算逻辑。返回实际删除的部门数量。
func DeleteManualDepartments(sqlDB *sql.DB) (int, error) {
	rows, err := sqlDB.Query(`SELECT id FROM departments WHERE wecom_dept_id=0`)
	if err != nil {
		return 0, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	deleted := 0
	for _, id := range ids {
		if err := DeleteDepartment(sqlDB, id); err != nil {
			return deleted, fmt.Errorf("删除手动部门 #%d: %w", id, err)
		}
		deleted++
	}
	return deleted, nil
}

func queryUserIDs(tx *sql.Tx, query string, args ...interface{}) ([]string, error) {
	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var userid string
		if err := rows.Scan(&userid); err != nil {
			return nil, err
		}
		out = append(out, userid)
	}
	return out, rows.Err()
}

// WecomDepartmentInput 供企业微信同步流程传入的部门原始信息（避免 db 包反向依赖 wecom 包）。
type WecomDepartmentInput struct {
	ID   int
	Name string
}

// UpsertWecomDepartments 将企业微信通讯录的部门结构导入/同步为本地部门表，
// 使部门以企业微信组织架构为唯一权威来源。规则：
//  1. 已通过 wecom_dept_id 关联过的部门：按企业微信最新名称更新（企业微信部门改名会自动同步）。
//  2. 尚未关联、但名称与某个"纯手动创建"的部门完全一致：自动关联，避免出现重复部门。
//  3. 其余：新建本地部门并关联。
//
// 返回值：企业微信部门 ID -> 本地部门 ID 的映射，供同步成员时写入 app_users.department_id。
func UpsertWecomDepartments(sqlDB *sql.DB, depts []WecomDepartmentInput) (map[int]int64, error) {
	tx, err := sqlDB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now().Format(time.RFC3339)
	result := make(map[int]int64, len(depts))

	for _, wd := range depts {
		if wd.ID <= 0 || strings.TrimSpace(wd.Name) == "" {
			continue
		}
		name := strings.TrimSpace(wd.Name)

		var localID int64
		err := tx.QueryRow(`SELECT id FROM departments WHERE wecom_dept_id=?`, wd.ID).Scan(&localID)
		if err == nil {
			// 已关联：名称以企业微信为准，保持最新。
			if _, err := tx.Exec(
				`UPDATE departments SET name=?, updated_at=? WHERE id=?`, name, now, localID,
			); err != nil {
				return nil, fmt.Errorf("更新企业微信部门 %s: %w", name, err)
			}
			result[wd.ID] = localID
			continue
		}
		if err != sql.ErrNoRows {
			return nil, err
		}

		// 未关联：按名称匹配现有的纯手动部门，自动关联，避免产生重复部门。
		err = tx.QueryRow(
			`SELECT id FROM departments WHERE name=? COLLATE NOCASE AND wecom_dept_id=0`, name,
		).Scan(&localID)
		if err == nil {
			if _, err := tx.Exec(
				`UPDATE departments SET wecom_dept_id=?, updated_at=? WHERE id=?`, wd.ID, now, localID,
			); err != nil {
				return nil, fmt.Errorf("关联企业微信部门 %s: %w", name, err)
			}
			result[wd.ID] = localID
			continue
		}
		if err != sql.ErrNoRows {
			return nil, err
		}

		// 全新部门：插入；名称与其他部门冲突时追加部门号加以区分。
		insertName := name
		res, insertErr := tx.Exec(
			`INSERT INTO departments (name, wecom_dept_id, updated_at) VALUES (?, ?, ?)`,
			insertName, wd.ID, now,
		)
		if insertErr != nil {
			insertName = fmt.Sprintf("%s（部门#%d）", name, wd.ID)
			res, insertErr = tx.Exec(
				`INSERT INTO departments (name, wecom_dept_id, updated_at) VALUES (?, ?, ?)`,
				insertName, wd.ID, now,
			)
			if insertErr != nil {
				return nil, fmt.Errorf("新建企业微信部门 %s: %w", name, insertErr)
			}
		}
		id, err := res.LastInsertId()
		if err != nil {
			return nil, err
		}
		result[wd.ID] = id
	}

	return result, tx.Commit()
}

// ListUsersByDepartmentID 返回指定部门的成员（按多对多关系表匹配，
// 因此一人挂多个部门时会同时出现在这几个部门的人员列表里）。
func ListUsersByDepartmentID(db *sql.DB, deptID int64) ([]AppUser, error) {
	rows, err := db.Query(
		`SELECT u.userid, u.name, u.mobile, u.departments, u.department_id,
		        COALESCE(d.name, ''), u.sources, u.updated_at
		 FROM app_users u
		 INNER JOIN app_user_departments ud ON ud.userid = u.userid AND ud.department_id = ?
		 LEFT JOIN departments d ON u.department_id = d.id
		 WHERE u.active=1
		 ORDER BY u.name, u.userid`,
		deptID,
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
