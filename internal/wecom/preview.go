package wecom

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
	"github.com/xiaoluoDD/my-backend-services/internal/logger"
)

// MainDeptPreviewItem 主部门对照条目（只读，不落库）。
type MainDeptPreviewItem struct {
	UserID               string             `json:"userid"`
	Name                 string             `json:"name"`
	MainDepartmentID     int64              `json:"main_department_id"`
	MainDepartmentName   string             `json:"main_department_name"`
	DepartmentIDs        []int64            `json:"department_ids"`
	DepartmentList       []db.DepartmentRef `json:"department_list"`
	WecomMainDepartment  int                `json:"wecom_main_department"`
	WecomDepartments     []int              `json:"wecom_departments"`
	LocalDepartmentID    int64              `json:"local_department_id"`
	LocalDepartmentName  string             `json:"local_department_name"`
	LocalDepartmentIDs   []int64            `json:"local_department_ids"`
	LocalDepartmentNames string             `json:"local_department_names"`
	HasMain              bool               `json:"has_main"`
	IsMulti              bool               `json:"is_multi"`
	MainMatchesLocal     bool               `json:"main_matches_local"`
	Sources              string             `json:"sources"`
}

// MainDeptPreviewResult 主部门对照结果。
type MainDeptPreviewResult struct {
	Users   []MainDeptPreviewItem `json:"users"`
	Summary map[string]int        `json:"summary"`
}

// PreviewMainDepartments 从企业微信实时拉取可见成员的主部门，只读对照，不写库。
func PreviewMainDepartments(sqlDB *sql.DB) (*MainDeptPreviewResult, error) {
	log := logger.Default()

	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	token, err := getToken(cfg)
	if err != nil {
		return nil, err
	}

	members, err := collectVisibleMembers(cfg, token)
	if err != nil {
		return nil, err
	}
	enrichMainDepartments(token, members) // 全量 user/get，拿真正的 main_department

	departments, err := FetchDepartments(token)
	if err != nil {
		log.Warn("preview: 拉取部门列表失败", "err", err)
		departments = nil
	}
	deptNames := make(map[int]string, len(departments))
	for _, d := range departments {
		if d.Name != "" {
			deptNames[d.ID] = d.Name
		}
	}

	wecomToLocal, localNames, err := db.MapExistingWecomDepartments(sqlDB)
	if err != nil {
		log.Warn("preview: 读取本地部门映射失败", "err", err)
		wecomToLocal = map[int]int64{}
		localNames = map[int64]string{}
	}

	localUsers, err := db.ListActiveUsers(sqlDB)
	if err != nil {
		log.Warn("preview: 读取本地成员失败", "err", err)
		localUsers = nil
	}
	localByID := make(map[string]db.AppUser, len(localUsers))
	for _, u := range localUsers {
		localByID[u.UserID] = u
	}

	items := make([]MainDeptPreviewItem, 0, len(members))
	withMain, withoutMain, multi, mismatch := 0, 0, 0, 0

	for _, m := range members {
		mainWecom := m.MainDepartment
		if mainWecom <= 0 && len(m.Departments) == 1 {
			mainWecom = m.Departments[0]
		}

		allWecom := append([]int(nil), m.Departments...)
		if mainWecom > 0 {
			allWecom = putIntFirst(allWecom, mainWecom)
		}

		deptList := make([]db.DepartmentRef, 0, len(allWecom))
		deptIDs := make([]int64, 0, len(allWecom))
		seenLocal := map[int64]struct{}{}
		for _, wid := range allWecom {
			name := deptNames[wid]
			if name == "" {
				name = fmt.Sprintf("企微部门#%d", wid)
			}
			var localID int64
			if lid, ok := wecomToLocal[wid]; ok && lid > 0 {
				localID = lid
				if n := localNames[lid]; n != "" {
					name = n
				}
			}
			deptList = append(deptList, db.DepartmentRef{ID: localID, Name: name})
			if localID > 0 {
				if _, dup := seenLocal[localID]; !dup {
					seenLocal[localID] = struct{}{}
					deptIDs = append(deptIDs, localID)
				}
			}
		}

		mainName := ""
		var mainLocalID int64
		if mainWecom > 0 {
			mainName = deptNames[mainWecom]
			if mainName == "" {
				mainName = fmt.Sprintf("企微部门#%d", mainWecom)
			}
			if lid, ok := wecomToLocal[mainWecom]; ok && lid > 0 {
				mainLocalID = lid
				if n := localNames[lid]; n != "" {
					mainName = n
				}
			}
		}

		local := localByID[m.UserID]
		localNamesJoined := ""
		if len(local.DepartmentList) > 0 {
			parts := make([]string, 0, len(local.DepartmentList))
			for _, d := range local.DepartmentList {
				parts = append(parts, d.Name)
			}
			localNamesJoined = joinNonEmpty(parts, "、")
		} else if local.DepartmentName != "" {
			localNamesJoined = local.DepartmentName
		}

		hasMain := mainWecom > 0
		isMulti := len(allWecom) > 1
		matches := true
		if local.UserID != "" && hasMain {
			matches = false
			if mainLocalID > 0 && local.DepartmentID > 0 {
				matches = mainLocalID == local.DepartmentID
			} else if mainName != "" && local.DepartmentName != "" {
				matches = local.DepartmentName == mainName ||
					(len(local.DepartmentList) > 0 && local.DepartmentList[0].Name == mainName)
			}
		}

		if hasMain {
			withMain++
		} else {
			withoutMain++
		}
		if isMulti {
			multi++
		}
		if local.UserID != "" && hasMain && !matches {
			mismatch++
		}

		name := m.Name
		if name == "" {
			name = m.UserID
		}
		items = append(items, MainDeptPreviewItem{
			UserID:               m.UserID,
			Name:                 name,
			MainDepartmentID:     mainLocalID,
			MainDepartmentName:   mainName,
			DepartmentIDs:        deptIDs,
			DepartmentList:       deptList,
			WecomMainDepartment:  mainWecom,
			WecomDepartments:     allWecom,
			LocalDepartmentID:    local.DepartmentID,
			LocalDepartmentName:  local.DepartmentName,
			LocalDepartmentIDs:   append([]int64(nil), local.DepartmentIDs...),
			LocalDepartmentNames: localNamesJoined,
			HasMain:              hasMain,
			IsMulti:              isMulti,
			MainMatchesLocal:     matches,
			Sources:              joinSources(m.Sources),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsMulti != items[j].IsMulti {
			return items[i].IsMulti
		}
		if items[i].MainMatchesLocal != items[j].MainMatchesLocal {
			return !items[i].MainMatchesLocal
		}
		if items[i].Name == items[j].Name {
			return items[i].UserID < items[j].UserID
		}
		return items[i].Name < items[j].Name
	})

	log.Info("preview main departments",
		"total", len(items), "with_main", withMain, "without_main", withoutMain,
		"multi", multi, "mismatch", mismatch)

	return &MainDeptPreviewResult{
		Users: items,
		Summary: map[string]int{
			"total":         len(items),
			"with_main":     withMain,
			"without_main":  withoutMain,
			"multi":         multi,
			"mismatch":      mismatch,
		},
	}, nil
}

// collectVisibleMembers 拉取应用可见范围内的成员（与同步同源，不写库）。
func collectVisibleMembers(cfg Config, token string) (map[string]*member, error) {
	scope, err := FetchAgentScope(cfg, token)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		for _, u := range ur.UserList {
			addUser(u.UserID, u.Name, u.Mobile, u.Department, fmt.Sprintf("party:%d", partyID))
		}
	}
	for _, tagID := range scope.AllowTags.TagID {
		tr, err := FetchTagMembers(token, tagID)
		if err != nil {
			return nil, err
		}
		for _, u := range tr.UserList {
			addUser(u.UserID, u.Name, "", nil, fmt.Sprintf("tag:%d", tagID))
		}
		for _, p := range tr.PartyList {
			ur, err := FetchDepartmentUsers(token, p.PartyID)
			if err != nil {
				return nil, err
			}
			for _, u := range ur.UserList {
				addUser(u.UserID, u.Name, u.Mobile, u.Department, fmt.Sprintf("tag:%d/party:%d", tagID, p.PartyID))
			}
		}
	}
	return members, nil
}

// enrichMainDepartments 对每个可见成员调用 user/get，拿到真正的 main_department。
func enrichMainDepartments(token string, members map[string]*member) {
	log := logger.Default()
	for userid, m := range members {
		detail, err := FetchUserDetail(token, userid)
		if err != nil {
			log.Warn("preview: user/get 失败", "userid", userid, "err", err)
			continue
		}
		if m.Name == "" && detail.Name != "" {
			m.Name = detail.Name
		}
		if m.Mobile == "" && detail.Mobile != "" {
			m.Mobile = detail.Mobile
		}
		if len(detail.Department) > 0 {
			m.Departments = detail.Department
		} else if detail.MainDepartment > 0 {
			m.Departments = []int{detail.MainDepartment}
		}
		m.MainDepartment = detail.MainDepartment
		log.Info("preview: user/get", "userid", userid,
			"dept_ids", len(m.Departments), "main_department", m.MainDepartment)
	}
}

func putIntFirst(ids []int, first int) []int {
	if first <= 0 {
		return ids
	}
	out := make([]int, 0, len(ids)+1)
	out = append(out, first)
	seen := map[int]struct{}{first: {}}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func joinNonEmpty(parts []string, sep string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}
