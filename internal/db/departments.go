package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Department 手动维护的部门。
type Department struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	UpdatedAt string `json:"updated_at"`
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
		`SELECT id, name, updated_at FROM departments ORDER BY name, id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Department
	for rows.Next() {
		var d Department
		if err := rows.Scan(&d.ID, &d.Name, &d.UpdatedAt); err != nil {
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
		`SELECT id, name, updated_at FROM departments WHERE id=?`, id,
	).Scan(&d.ID, &d.Name, &d.UpdatedAt)
	return d, err
}

// CreateDepartment 新建部门。
func CreateDepartment(db *sql.DB, name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("部门名称不能为空")
	}
	now := time.Now().Format(time.RFC3339)
	res, err := db.Exec(
		`INSERT INTO departments (name, updated_at) VALUES (?, ?)`, name, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateDepartment 更新部门名称。
func UpdateDepartment(db *sql.DB, id int64, name string) error {
	if id <= 0 {
		return fmt.Errorf("无效的部门 ID")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("部门名称不能为空")
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

// DeleteDepartment 删除部门，并将关联成员的 department_id 清零。
func DeleteDepartment(db *sql.DB, id int64) error {
	if id <= 0 {
		return fmt.Errorf("无效的部门 ID")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE app_users SET department_id=0, departments='' WHERE department_id=?`, id,
	); err != nil {
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
	return tx.Commit()
}

// ListUsersByDepartmentID 返回指定部门的成员。
func ListUsersByDepartmentID(db *sql.DB, deptID int64) ([]AppUser, error) {
	rows, err := db.Query(
		`SELECT u.userid, u.name, u.mobile, u.departments, u.department_id,
		        COALESCE(d.name, ''), u.sources, u.updated_at
		 FROM app_users u
		 LEFT JOIN departments d ON u.department_id = d.id
		 WHERE u.active=1 AND u.department_id=?
		 ORDER BY u.name, u.userid`,
		deptID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAppUsers(rows)
}
