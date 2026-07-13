package db

import "time"

const (
	ProjectStatusNotStarted = "待启动"
	ProjectStatusInProgress = "进行中"
	ProjectStatusOverdue    = "逾期"
	ProjectStatusCompleted  = "已完结"
)

// EffectiveProjectStatus 根据项目启动日期与实际完结日期计算基础状态（不含子任务逾期）。
func EffectiveProjectStatus(p Project) string {
	if normalizeDateString(p.EndDate) != "" {
		return ProjectStatusCompleted
	}
	if start, ok := parseDateOnly(p.StartDate); ok && dateOnly(time.Now()).Before(start) {
		return ProjectStatusNotStarted
	}
	return ProjectStatusInProgress
}

// DisplayProjectStatus 返回项目对外展示状态（含子任务逾期判定）。
func DisplayProjectStatus(p Project, subtasks []ProjectSubtask) string {
	base := EffectiveProjectStatus(p)
	if base == ProjectStatusCompleted || base == ProjectStatusNotStarted {
		return base
	}
	for i := range subtasks {
		if EffectiveSubtaskStatus(subtasks[i]) == SubtaskStatusOverdue {
			return ProjectStatusOverdue
		}
	}
	return ProjectStatusInProgress
}

// DisplayProjectStatusFromOverdueFlag 在仅有 subtask_any_overdue 汇总时使用。
func DisplayProjectStatusFromOverdueFlag(p Project, subtaskAnyOverdue bool) string {
	base := EffectiveProjectStatus(p)
	if base == ProjectStatusCompleted || base == ProjectStatusNotStarted {
		return base
	}
	if subtaskAnyOverdue {
		return ProjectStatusOverdue
	}
	return ProjectStatusInProgress
}

// SyncProjectStatus 将 status 字段与展示规则对齐。
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
