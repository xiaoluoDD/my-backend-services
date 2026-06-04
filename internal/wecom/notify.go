package wecom

import (
	"fmt"
	"strings"
	"time"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
)

// FormatProjectReminder 生成带项目关键信息的提醒正文。
func FormatProjectReminder(p db.Project, extra string) string {
	var b strings.Builder
	b.WriteString("📢 项目提醒\n")
	if extra != "" {
		b.WriteString(extra)
		if !strings.HasSuffix(extra, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("────────────────\n")
	}
	b.WriteString(fmt.Sprintf("项目名称：%s\n", emptyDash(p.Name)))
	if p.Year != "" || p.WorkNo != "" {
		b.WriteString(fmt.Sprintf("年度 / 工番号：%s / %s\n", emptyDash(p.Year), emptyDash(p.WorkNo)))
	}
	b.WriteString(fmt.Sprintf("项目状态：%s\n", emptyDash(p.Status)))
	if p.ManagerName != "" || p.ManagerUserID != "" {
		mgr := p.ManagerName
		if mgr == "" {
			mgr = p.ManagerUserID
		} else if p.ManagerUserID != "" {
			mgr = fmt.Sprintf("%s（%s）", p.ManagerName, p.ManagerUserID)
		}
		b.WriteString(fmt.Sprintf("负责人：%s\n", mgr))
	}
	if p.StartDate != "" || p.EndDate != "" {
		b.WriteString(fmt.Sprintf("周期：%s ～ %s\n", emptyDash(p.StartDate), emptyDash(p.EndDate)))
	}
	if strings.TrimSpace(p.Tasks) != "" {
		b.WriteString(fmt.Sprintf("项目任务：%s\n", strings.TrimSpace(p.Tasks)))
	}
	b.WriteString("────────────────\n")
	b.WriteString("发送时间：" + time.Now().Format("2006-01-02 15:04:05"))
	return b.String()
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return strings.TrimSpace(s)
}

// NotifyProjectMembers 向项目内成员（负责人 + 关联成员）发送提醒。
func NotifyProjectMembers(p db.Project, members []db.ProjectMember, extra string) (msgID string, recipients []db.ProjectMember, content string, err error) {
	recipients = db.ProjectRecipients(p, members)
	if len(recipients) == 0 {
		return "", nil, "", fmt.Errorf("该项目未配置负责人或项目成员，请先编辑项目")
	}

	cfg, err := LoadConfig()
	if err != nil {
		return "", nil, "", err
	}

	userids := make([]string, 0, len(recipients))
	for _, r := range recipients {
		userids = append(userids, r.UserID)
	}

	content = FormatProjectReminder(p, extra)
	msgID, err = SendTextToUsers(cfg, userids, content)
	return msgID, recipients, content, err
}
