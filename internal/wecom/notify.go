package wecom

import (
	"fmt"
	"strings"
	"time"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
)

// FormatProjectReminder 生成带项目关键信息的提醒正文。
func FormatProjectReminder(p db.Project, extra string) string {
	return FormatProjectReminderEx(p, db.ProjectSubtaskStats{}, extra)
}

// FormatProjectReminderEx 生成提醒正文，优先使用子任务汇总信息。
func FormatProjectReminderEx(p db.Project, stats db.ProjectSubtaskStats, extra string) string {
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
	b.WriteString(fmt.Sprintf("项目状态：%s\n", emptyDash(db.EffectiveProjectStatus(p))))
	if p.ManagerName != "" || p.ManagerUserID != "" {
		mgr := p.ManagerName
		if mgr == "" {
			mgr = p.ManagerUserID
		} else if p.ManagerUserID != "" {
			mgr = fmt.Sprintf("%s（%s）", p.ManagerName, p.ManagerUserID)
		}
		b.WriteString(fmt.Sprintf("负责人：%s\n", mgr))
	}

	b.WriteString(fmt.Sprintf("启动日期：%s\n", emptyDash(p.StartDate)))
	if p.EndDate != "" {
		b.WriteString(fmt.Sprintf("实际完结日期：%s\n", emptyDash(p.EndDate)))
	}

	tasks := strings.TrimSpace(stats.TaskSummary)
	if tasks == "" {
		tasks = strings.TrimSpace(p.Tasks)
	}
	if tasks != "" {
		b.WriteString(fmt.Sprintf("项目任务：%s\n", tasks))
	}
	b.WriteString("────────────────\n")
	b.WriteString("发送时间：" + time.Now().Format("2006-01-02 15:04:05"))
	return b.String()
}

// FormatScheduledReminderHeader 定时提醒的标题说明行。daysRemaining 为距计划日剩余天数（当天为 0）。
func FormatScheduledReminderHeader(kind string, daysRemaining int, eventDate string) string {
	dateText := emptyDash(eventDate)
	switch kind {
	case db.ReminderKindStart:
		if daysRemaining <= 0 {
			return fmt.Sprintf("【项目启动提醒】今日为计划启动日（%s）", dateText)
		}
		return fmt.Sprintf("【项目启动提醒】距离计划启动还有 %d 天（计划启动：%s）", daysRemaining, dateText)
	default:
		return ""
	}
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return strings.TrimSpace(s)
}

// NotifyProjectManager 向项目负责人发送提醒。
func NotifyProjectManager(p db.Project, extra string) (msgID string, recipients []db.ProjectMember, content string, err error) {
	return NotifyProjectManagerEx(p, db.ProjectSubtaskStats{}, extra)
}

// NotifyProjectManagerEx 向项目负责人发送提醒，正文中可包含子任务汇总信息。
func NotifyProjectManagerEx(p db.Project, stats db.ProjectSubtaskStats, extra string) (msgID string, recipients []db.ProjectMember, content string, err error) {
	recipients, err = db.ProjectManagerRecipient(p)
	if err != nil {
		return "", nil, "", err
	}

	cfg, err := LoadConfig()
	if err != nil {
		return "", nil, "", err
	}

	userids := []string{recipients[0].UserID}
	content = FormatProjectReminderEx(p, stats, extra)
	msgID, err = SendTextToUsers(cfg, userids, content)
	return msgID, recipients, content, err
}

// NotifyProjectMembers 向项目内成员（负责人 + 关联成员）发送提醒。
func NotifyProjectMembers(p db.Project, members []db.ProjectMember, extra string) (msgID string, recipients []db.ProjectMember, content string, err error) {
	return NotifyProjectMembersEx(p, members, db.ProjectSubtaskStats{}, extra)
}

// NotifyProjectMembersEx 发送提醒，正文中可包含子任务汇总信息。
func NotifyProjectMembersEx(p db.Project, members []db.ProjectMember, stats db.ProjectSubtaskStats, extra string) (msgID string, recipients []db.ProjectMember, content string, err error) {
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

	content = FormatProjectReminderEx(p, stats, extra)
	msgID, err = SendTextToUsers(cfg, userids, content)
	return msgID, recipients, content, err
}
