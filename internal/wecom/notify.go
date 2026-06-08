package wecom

import (
	"fmt"
	"strings"
	"time"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
)

// ProjectDigestLine 项目摘要行。
type ProjectDigestLine struct {
	Name          string
	WorkNo        string
	ManagerName   string
	Status        string
	EventDate     string
	DaysRemaining int
}

// SubtaskDigestLine 子任务摘要行。
type SubtaskDigestLine struct {
	ProjectName      string
	WorkNo           string
	ManagerName      string
	Content          string
	PlannedStartDate string
	EventDate        string
	PlannedEndDate   string
	DaysRemaining    int
}

// FormatProjectReminder 生成项目提醒正文。
func FormatProjectReminder(p db.Project, extra string) string {
	return FormatCompactProjectReminder(p, extra)
}

// FormatCompactProjectReminder 单项目精简提醒（手动触发等场景）。
func FormatCompactProjectReminder(p db.Project, header string) string {
	var b strings.Builder
	b.WriteString("📢 项目提醒\n")
	if header != "" {
		b.WriteString(header)
		b.WriteByte('\n')
	}
	b.WriteString("════════════════\n")
	b.WriteString(formatProjectBlock(projectDigestFromProject(p, "", 0), ""))
	b.WriteString("════════════════\n")
	b.WriteString("发送时间：" + time.Now().Format("2006-01-02 15:04:05"))
	return b.String()
}

func projectDigestFromProject(p db.Project, eventDate string, daysRemaining int) ProjectDigestLine {
	mgr := strings.TrimSpace(p.ManagerName)
	if mgr == "" {
		mgr = strings.TrimSpace(p.ManagerUserID)
	}
	return ProjectDigestLine{
		Name:          p.Name,
		WorkNo:        p.WorkNo,
		ManagerName:   mgr,
		Status:        db.EffectiveProjectStatus(p),
		EventDate:     eventDate,
		DaysRemaining: daysRemaining,
	}
}

// FormatProjectStartDigest 项目负责人：项目启动摘要。
func FormatProjectStartDigest(lines []ProjectDigestLine) string {
	return formatProjectDigest("📢 项目启动提醒", "计划启动", lines)
}

// FormatProjectEndDigest 项目负责人：项目完结摘要。
func FormatProjectEndDigest(lines []ProjectDigestLine) string {
	return formatProjectDigest("📢 项目完结提醒", "计划完结", lines)
}

func formatProjectDigest(title, eventLabel string, lines []ProjectDigestLine) string {
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n════════════════\n")
	b.WriteString(fmt.Sprintf("共 %d 个项目需关注\n\n", len(lines)))
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(formatProjectBlock(line, eventLabel))
	}
	b.WriteString("\n════════════════\n")
	b.WriteString("发送时间：" + time.Now().Format("2006-01-02 15:04:05"))
	return b.String()
}

func formatProjectBlock(line ProjectDigestLine, eventLabel string) string {
	var b strings.Builder
	b.WriteString("▎")
	b.WriteString(formatProjectTitle(line.Name, line.WorkNo))
	b.WriteByte('\n')
	if line.ManagerName != "" {
		b.WriteString(fmt.Sprintf("  负责人：%s\n", line.ManagerName))
	}
	if line.Status != "" {
		b.WriteString(fmt.Sprintf("  状态：%s\n", line.Status))
	}
	if line.EventDate != "" && eventLabel != "" {
		b.WriteString(fmt.Sprintf("  %s：%s\n", eventLabel, line.EventDate))
		b.WriteString(fmt.Sprintf("  ⏰ %s\n", formatCountdown(line.DaysRemaining, line.EventDate)))
	}
	return b.String()
}

// FormatManagerSubtaskStartDigest 项目负责人：子任务计划开始摘要。
func FormatManagerSubtaskStartDigest(projectName, workNo, managerName string, lines []SubtaskDigestLine) string {
	return formatManagerSubtaskDigest("📢 子任务提醒 · 计划开始", projectName, workNo, managerName, lines, formatSubtaskStartBlock)
}

// FormatMemberSubtaskStartDigest 子项目成员：个人子任务计划开始摘要。
func FormatMemberSubtaskStartDigest(lines []SubtaskDigestLine) string {
	return formatMemberSubtaskDigest("📢 子任务提醒 · 计划开始", lines, formatSubtaskStartBlock)
}

// FormatManagerSubtaskEndDigest 项目负责人：子任务计划完结摘要。
func FormatManagerSubtaskEndDigest(projectName, workNo, managerName string, lines []SubtaskDigestLine) string {
	return formatManagerSubtaskDigest("📢 子任务提醒 · 计划完结", projectName, workNo, managerName, lines, formatSubtaskEndBlock)
}

// FormatMemberSubtaskEndDigest 子项目成员：个人子任务计划完结摘要。
func FormatMemberSubtaskEndDigest(lines []SubtaskDigestLine) string {
	return formatMemberSubtaskDigest("📢 子任务提醒 · 计划完结", lines, formatSubtaskEndBlock)
}

// FormatManagerSubtaskDigest 兼容旧名。
func FormatManagerSubtaskDigest(projectName, workNo, managerName string, lines []SubtaskDigestLine) string {
	return FormatManagerSubtaskStartDigest(projectName, workNo, managerName, lines)
}

// FormatMemberSubtaskDigest 兼容旧名。
func FormatMemberSubtaskDigest(lines []SubtaskDigestLine) string {
	return FormatMemberSubtaskStartDigest(lines)
}

func formatManagerSubtaskDigest(title, projectName, workNo, managerName string, lines []SubtaskDigestLine, blockFn func(int, SubtaskDigestLine) string) string {
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n════════════════\n")
	b.WriteString("▎")
	b.WriteString(formatProjectTitle(projectName, workNo))
	b.WriteByte('\n')
	if managerName != "" {
		b.WriteString(fmt.Sprintf("  项目负责人：%s\n", managerName))
	}
	b.WriteString(fmt.Sprintf("\n共 %d 项子任务需关注\n\n", len(lines)))
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(blockFn(i+1, line))
	}
	b.WriteString("\n════════════════\n")
	b.WriteString("发送时间：" + time.Now().Format("2006-01-02 15:04:05"))
	return b.String()
}

func formatMemberSubtaskDigest(title string, lines []SubtaskDigestLine, blockFn func(int, SubtaskDigestLine) string) string {
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n════════════════\n")
	b.WriteString(fmt.Sprintf("共 %d 项需关注\n\n", len(lines)))
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(blockFn(i+1, line))
	}
	b.WriteString("\n════════════════\n")
	b.WriteString("发送时间：" + time.Now().Format("2006-01-02 15:04:05"))
	return b.String()
}

func formatSubtaskStartBlock(index int, line SubtaskDigestLine) string {
	return formatSubtaskBlock(index, line, "计划开始", line.EventDate, line.DaysRemaining, true)
}

func formatSubtaskEndBlock(index int, line SubtaskDigestLine) string {
	return formatSubtaskBlock(index, line, "计划完结", line.EventDate, line.DaysRemaining, false)
}

func formatSubtaskBlock(index int, line SubtaskDigestLine, eventLabel, eventDate string, daysRemaining int, showPlannedEnd bool) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("  %d. ", index))
	if line.ProjectName != "" {
		b.WriteString(formatProjectTitle(line.ProjectName, line.WorkNo))
		b.WriteByte('\n')
		if line.ManagerName != "" {
			b.WriteString(fmt.Sprintf("     项目负责人：%s\n", line.ManagerName))
		}
	}
	content := emptyDash(line.Content)
	b.WriteString(fmt.Sprintf("     任务：%s\n", content))
	if plannedStart := strings.TrimSpace(line.PlannedStartDate); plannedStart != "" && !showPlannedEnd {
		b.WriteString(fmt.Sprintf("     计划开始：%s\n", plannedStart))
	}
	if eventDate != "" && eventLabel != "" {
		b.WriteString(fmt.Sprintf("     %s：%s\n", eventLabel, eventDate))
		b.WriteString(fmt.Sprintf("     ⏰ %s\n", formatCountdown(daysRemaining, eventDate)))
	}
	if showPlannedEnd {
		if plannedEnd := strings.TrimSpace(line.PlannedEndDate); plannedEnd != "" {
			b.WriteString(fmt.Sprintf("     计划完结：%s\n", plannedEnd))
		} else {
			b.WriteString("     计划完结：—\n")
		}
	}
	return b.String()
}

func formatProjectTitle(name, workNo string) string {
	name = strings.TrimSpace(name)
	workNo = strings.TrimSpace(workNo)
	if name == "" && workNo == "" {
		return "—"
	}
	if workNo == "" {
		return name
	}
	if name == "" {
		return workNo
	}
	return fmt.Sprintf("%s（%s）", name, workNo)
}

func formatCountdown(daysRemaining int, eventDate string) string {
	dateText := emptyDash(eventDate)
	if daysRemaining <= 0 {
		return fmt.Sprintf("今日为计划日（%s）", dateText)
	}
	return fmt.Sprintf("距离计划日还有 %d 天（%s）", daysRemaining, dateText)
}

// FormatScheduledReminderHeader 定时提醒标题行（手动/API 兼容）。
func FormatScheduledReminderHeader(kind string, daysRemaining int, eventDate string) string {
	dateText := emptyDash(eventDate)
	switch kind {
	case db.ReminderKindStart:
		if daysRemaining <= 0 {
			return fmt.Sprintf("【项目启动提醒】今日为计划启动日（%s）", dateText)
		}
		return fmt.Sprintf("【项目启动提醒】距离计划启动还有 %d 天（%s）", daysRemaining, dateText)
	case db.ReminderKindEnd:
		if daysRemaining <= 0 {
			return fmt.Sprintf("【项目完结提醒】今日为计划完结日（%s）", dateText)
		}
		return fmt.Sprintf("【项目完结提醒】距离计划完结还有 %d 天（%s）", daysRemaining, dateText)
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
	recipients, err = db.ProjectManagerRecipient(p)
	if err != nil {
		return "", nil, "", err
	}
	cfg, err := LoadConfig()
	if err != nil {
		return "", nil, "", err
	}
	content = FormatCompactProjectReminder(p, extra)
	msgID, err = SendTextToUsers(cfg, []string{recipients[0].UserID}, content)
	return msgID, recipients, content, err
}

// NotifyUsers 向指定用户发送文本。
func NotifyUsers(userIDs []string, content string) (msgID string, err error) {
	if len(userIDs) == 0 {
		return "", fmt.Errorf("无有效收件人")
	}
	cfg, err := LoadConfig()
	if err != nil {
		return "", err
	}
	return SendTextToUsers(cfg, userIDs, content)
}

// NotifyProjectMembers 向项目内成员发送提醒。
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
	content = FormatCompactProjectReminder(p, extra)
	msgID, err = SendTextToUsers(cfg, userids, content)
	return msgID, recipients, content, err
}

func FormatProjectReminderEx(p db.Project, stats db.ProjectSubtaskStats, extra string) string {
	_ = stats
	return FormatCompactProjectReminder(p, extra)
}

func NotifyProjectManagerEx(p db.Project, stats db.ProjectSubtaskStats, extra string) (msgID string, recipients []db.ProjectMember, content string, err error) {
	_ = stats
	return NotifyProjectManager(p, extra)
}

func NotifyProjectMembersEx(p db.Project, members []db.ProjectMember, stats db.ProjectSubtaskStats, extra string) (msgID string, recipients []db.ProjectMember, content string, err error) {
	_ = stats
	return NotifyProjectMembers(p, members, extra)
}
