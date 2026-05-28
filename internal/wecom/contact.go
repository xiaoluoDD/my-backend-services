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

func joinDepartments(depts []int) string {
	if len(depts) == 0 {
		return "[]"
	}
	parts := make([]string, len(depts))
	for i, d := range depts {
		parts[i] = strconv.Itoa(d)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
