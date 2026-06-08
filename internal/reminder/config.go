package reminder

// 提醒相关调试开关（软件定版后改回「定版值」）。
const (
	// oncePerDayReminderRun 定版值 true：同一自然日仅在 reminder_time 对应分钟触发扫描一次。
	oncePerDayReminderRun = false

	// oncePerDayReminderDedup 定版值 true：同一项目、同类型（启动/完结）提醒每天只发一次。
	oncePerDayReminderDedup = false

	// schedulerMinuteLog 定版值 false：每分钟输出一次 tick 日志，便于确认定时器在跑。
	schedulerMinuteLog = true
)
