package wecom

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type baseResp struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func apiGET(token, apiPath string, query url.Values) ([]byte, error) {
	u, err := url.Parse("https://qyapi.weixin.qq.com" + apiPath)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("access_token", token)
	for k, vs := range query {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

type agentGetResp struct {
	baseResp
	AllowUserInfos struct {
		User []struct {
			UserID string `json:"userid"`
			Name   string `json:"name"`
		} `json:"user"`
	} `json:"allow_userinfos"`
	AllowParties struct {
		PartyID []int `json:"partyid"`
	} `json:"allow_parties"`
	AllowTags struct {
		TagID []int `json:"tagid"`
	} `json:"allow_tags"`
}

// FetchAgentScope 获取自建应用可见范围。
func FetchAgentScope(cfg Config, token string) (*agentGetResp, error) {
	body, err := apiGET(token, "/cgi-bin/agent/get", url.Values{
		"agentid": []string{strconv.Itoa(cfg.AgentID)},
	})
	if err != nil {
		return nil, err
	}
	var ar agentGetResp
	if err := json.Unmarshal(body, &ar); err != nil {
		return nil, fmt.Errorf("解析 agent/get: %w", err)
	}
	if ar.ErrCode != 0 {
		return nil, fmt.Errorf("agent/get 失败: errcode=%d errmsg=%s", ar.ErrCode, ar.ErrMsg)
	}
	return &ar, nil
}

type userListResp struct {
	baseResp
	UserList []struct {
		UserID     string `json:"userid"`
		Name       string `json:"name"`
		Mobile     string `json:"mobile"`
		Department []int  `json:"department"`
	} `json:"userlist"`
}

// FetchDepartmentUsers 获取部门成员（含子部门）。
func FetchDepartmentUsers(token string, departmentID int) (*userListResp, error) {
	body, err := apiGET(token, "/cgi-bin/user/list", url.Values{
		"department_id": []string{strconv.Itoa(departmentID)},
		"fetch_child":   []string{"1"},
	})
	if err != nil {
		return nil, err
	}
	var ur userListResp
	if err := json.Unmarshal(body, &ur); err != nil {
		return nil, fmt.Errorf("解析 user/list: %w", err)
	}
	if ur.ErrCode != 0 {
		return nil, fmt.Errorf("user/list(dept=%d) 失败: errcode=%d errmsg=%s",
			departmentID, ur.ErrCode, ur.ErrMsg)
	}
	return &ur, nil
}

type tagGetResp struct {
	baseResp
	UserList []struct {
		UserID string `json:"userid"`
		Name   string `json:"name"`
	} `json:"userlist"`
	PartyList []struct {
		PartyID int `json:"partyid"`
	} `json:"partylist"`
}

type userGetResp struct {
	baseResp
	UserID     string `json:"userid"`
	Name       string `json:"name"`
	Mobile     string `json:"mobile"`
	Department []int  `json:"department"`
}

// FetchUserDetail 读取单个成员详情（用于补全姓名）。
func FetchUserDetail(token, userid string) (*userGetResp, error) {
	body, err := apiGET(token, "/cgi-bin/user/get", url.Values{
		"userid": []string{userid},
	})
	if err != nil {
		return nil, err
	}
	var ur userGetResp
	if err := json.Unmarshal(body, &ur); err != nil {
		return nil, fmt.Errorf("解析 user/get: %w", err)
	}
	if ur.ErrCode != 0 {
		return nil, fmt.Errorf("user/get(%s) 失败: errcode=%d errmsg=%s", userid, ur.ErrCode, ur.ErrMsg)
	}
	return &ur, nil
}

// FetchTagMembers 获取标签成员。
func FetchTagMembers(token string, tagID int) (*tagGetResp, error) {
	body, err := apiGET(token, "/cgi-bin/tag/get", url.Values{
		"tagid": []string{strconv.Itoa(tagID)},
	})
	if err != nil {
		return nil, err
	}
	var tr tagGetResp
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("解析 tag/get: %w", err)
	}
	if tr.ErrCode != 0 {
		return nil, fmt.Errorf("tag/get(%d) 失败: errcode=%d errmsg=%s", tagID, tr.ErrCode, tr.ErrMsg)
	}
	return &tr, nil
}

// member 同步过程中的成员聚合。
type member struct {
	UserID      string
	Name        string
	Mobile      string
	Departments []int
	Sources     []string
}

func (m *member) addSource(src string) {
	for _, s := range m.Sources {
		if s == src {
			return
		}
	}
	m.Sources = append(m.Sources, src)
}

func joinSources(sources []string) string {
	return strings.Join(sources, ",")
}

type departmentListResp struct {
	baseResp
	Department []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		ParentID int    `json:"parentid"`
	} `json:"department"`
}

// FetchDepartmentList 拉取企业部门 id→名称（需「通讯录-部门信息只读」权限）。
func FetchDepartmentList(token string) (map[int]string, error) {
	body, err := apiGET(token, "/cgi-bin/department/list", nil)
	if err != nil {
		return nil, err
	}
	var dr departmentListResp
	if err := json.Unmarshal(body, &dr); err != nil {
		return nil, fmt.Errorf("解析 department/list: %w", err)
	}
	if dr.ErrCode != 0 {
		return nil, fmt.Errorf("department/list 失败: errcode=%d errmsg=%s", dr.ErrCode, dr.ErrMsg)
	}
	out := make(map[int]string, len(dr.Department))
	for _, d := range dr.Department {
		if d.ID > 0 && d.Name != "" {
			out[d.ID] = d.Name
		}
	}
	return out, nil
}

// FormatDepartmentNames 将成员所属部门 id 列表转为可读名称。
func FormatDepartmentNames(depts []int, names map[int]string) string {
	if len(depts) == 0 {
		return ""
	}
	seen := make(map[int]struct{}, len(depts))
	parts := make([]string, 0, len(depts))
	for _, id := range depts {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if name, ok := names[id]; ok && name != "" {
			parts = append(parts, name)
		} else {
			parts = append(parts, fmt.Sprintf("部门#%d", id))
		}
	}
	return strings.Join(parts, "、")
}
