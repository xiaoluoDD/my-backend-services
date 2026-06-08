package db

import "time"

const (
	ProjectStatusNotStarted = "待启动"
	ProjectStatusInProgress = "进行中"
	ProjectStatusCompleted  = "已完结"
)

// EffectiveProjectStatus 根据项目启动日期与实际完结日期计算展示状态（与子任务无关）。
func EffectiveProjectStatus(p Project) string {
	if normalizeDateString(p.EndDate) != "" {
		return ProjectStatusCompleted
	}
	if start, ok := parseDateOnly(p.StartDate); ok && dateOnly(time.Now()).Before(start) {
		return ProjectStatusNotStarted
	}
	return ProjectStatusInProgress
}

// SyncProjectStatus 将 status 字段与日期规则对齐。
func SyncProjectStatus(p *Project) {
	if p == nil {
		return
	}
	p.Status = EffectiveProjectStatus(*p)
}

// ProjectHasActualEnd 项目是否已填入实际完结日期。
func ProjectHasActualEnd(p Project) bool {
	return normalizeDateString(p.EndDate) != ""
}

// ProjectHasStarted 当前日期不早于项目启动日期。
func ProjectHasStarted(p Project) bool {
	start, ok := parseDateOnly(p.StartDate)
	if !ok {
		return false
	}
	return !dateOnly(time.Now()).Before(start)
}
