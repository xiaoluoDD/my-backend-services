package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
	"github.com/xiaoluoDD/my-backend-services/internal/wecom"
)

const joinNotifyDebounce = 3 * time.Second

type pendingJoinNotify struct {
	project  db.Project
	stats    db.ProjectSubtaskStats
	member   db.ProjectMember
	subtasks []wecom.JoinSubtaskBrief
	timer    *time.Timer
}

type joinNotifyCoordinator struct {
	mu      sync.Mutex
	pending map[string]*pendingJoinNotify
}

var memberJoinCoordinator = &joinNotifyCoordinator{
	pending: make(map[string]*pendingJoinNotify),
}

func joinNotifyKey(projectID int64, userID string) string {
	return fmt.Sprintf("%d:%s", projectID, userID)
}

func addedProjectMembers(before, after []db.ProjectMember) []db.ProjectMember {
	beforeSet := memberUserIDSet(before)
	var added []db.ProjectMember
	for _, m := range after {
		if m.UserID == "" {
			continue
		}
		if _, ok := beforeSet[m.UserID]; !ok {
			added = append(added, m)
		}
	}
	return added
}

func addedSubtaskMembers(before, after []db.ProjectMember) []db.ProjectMember {
	return addedProjectMembers(before, after)
}

func memberUserIDSet(members []db.ProjectMember) map[string]struct{} {
	set := make(map[string]struct{}, len(members))
	for _, m := range members {
		if m.UserID != "" {
			set[m.UserID] = struct{}{}
		}
	}
	return set
}

func shouldSkipJoinNotify(project db.Project, member db.ProjectMember) bool {
	if member.UserID == "" {
		return true
	}
	return member.UserID == project.ManagerUserID
}

func loadProjectJoinContext(projectID int64) (db.Project, db.ProjectSubtaskStats, error) {
	project, err := db.GetProject(sqlDB, projectID)
	if err != nil {
		return db.Project{}, db.ProjectSubtaskStats{}, err
	}
	stats, err := db.SummarizeProjectSubtaskStats(sqlDB, projectID)
	if err != nil {
		return db.Project{}, db.ProjectSubtaskStats{}, err
	}
	return project, stats, nil
}

func notifyNewExplicitProjectMembers(projectID int64, added []db.ProjectMember) {
	if len(added) == 0 {
		return
	}
	project, stats, err := loadProjectJoinContext(projectID)
	if err != nil {
		return
	}
	for _, member := range added {
		if shouldSkipJoinNotify(project, member) {
			continue
		}
		memberJoinCoordinator.scheduleProjectJoin(project, stats, member)
	}
}

func notifyNewSubtaskMembers(projectID int64, subtask db.ProjectSubtask, added []db.ProjectMember, wasOnProject map[string]struct{}) {
	if len(added) == 0 {
		return
	}
	project, stats, err := loadProjectJoinContext(projectID)
	if err != nil {
		return
	}
	brief := wecom.JoinSubtaskBriefFromModel(subtask)
	for _, member := range added {
		if shouldSkipJoinNotify(project, member) {
			continue
		}
		if _, onProject := wasOnProject[member.UserID]; onProject {
			if memberJoinCoordinator.addSubtaskJoin(project, stats, member, brief) {
				continue
			}
			content := wecom.FormatMemberJoinSubtask(project, brief)
			wecom.NotifyMemberJoinAsync(member.UserID, content)
			continue
		}
		content := wecom.FormatMemberJoinProjectAndSubtasks(project, stats, []wecom.JoinSubtaskBrief{brief})
		wecom.NotifyMemberJoinAsync(member.UserID, content)
	}
}

func projectMemberSnapshot(projectID int64) map[string]struct{} {
	set, err := db.ProjectMemberUserIDSet(sqlDB, projectID)
	if err != nil {
		return map[string]struct{}{}
	}
	return set
}

func (c *joinNotifyCoordinator) scheduleProjectJoin(project db.Project, stats db.ProjectSubtaskStats, member db.ProjectMember) {
	key := joinNotifyKey(project.ID, member.UserID)
	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.pending[key]; ok {
		existing.project = project
		existing.stats = stats
		existing.member = member
		if existing.timer != nil {
			existing.timer.Stop()
		}
		existing.timer = time.AfterFunc(joinNotifyDebounce, func() {
			c.flush(key)
		})
		return
	}

	pending := &pendingJoinNotify{
		project: project,
		stats:   stats,
		member:  member,
	}
	pending.timer = time.AfterFunc(joinNotifyDebounce, func() {
		c.flush(key)
	})
	c.pending[key] = pending
}

func (c *joinNotifyCoordinator) addSubtaskJoin(project db.Project, stats db.ProjectSubtaskStats, member db.ProjectMember, brief wecom.JoinSubtaskBrief) bool {
	key := joinNotifyKey(project.ID, member.UserID)
	c.mu.Lock()
	defer c.mu.Unlock()

	existing, ok := c.pending[key]
	if !ok {
		return false
	}

	existing.project = project
	existing.stats = stats
	existing.member = member
	if !containsJoinSubtask(existing.subtasks, brief) {
		existing.subtasks = append(existing.subtasks, brief)
	}
	if existing.timer != nil {
		existing.timer.Stop()
	}
	existing.timer = time.AfterFunc(joinNotifyDebounce, func() {
		c.flush(key)
	})
	return true
}

func containsJoinSubtask(list []wecom.JoinSubtaskBrief, brief wecom.JoinSubtaskBrief) bool {
	for _, item := range list {
		if item.Content == brief.Content &&
			item.PlannedStartDate == brief.PlannedStartDate &&
			item.PlannedEndDate == brief.PlannedEndDate {
			return true
		}
	}
	return false
}

func (c *joinNotifyCoordinator) flush(key string) {
	c.mu.Lock()
	pending, ok := c.pending[key]
	if !ok {
		c.mu.Unlock()
		return
	}
	delete(c.pending, key)
	c.mu.Unlock()

	if pending == nil || pending.member.UserID == "" {
		return
	}

	var content string
	if len(pending.subtasks) > 0 {
		content = wecom.FormatMemberJoinProjectAndSubtasks(pending.project, pending.stats, pending.subtasks)
	} else {
		content = wecom.FormatMemberJoinProject(pending.project, pending.stats)
	}
	wecom.NotifyMemberJoinAsync(pending.member.UserID, content)
}
