package db

import "testing"

func TestDisplayProjectStatusFromOverdueFlag(t *testing.T) {
	p := Project{StartDate: "2020-01-01"}
	if got := DisplayProjectStatusFromOverdueFlag(p, false); got != ProjectStatusInProgress {
		t.Fatalf("expected in progress, got %s", got)
	}
	if got := DisplayProjectStatusFromOverdueFlag(p, true); got != ProjectStatusOverdue {
		t.Fatalf("expected overdue, got %s", got)
	}

	p.EndDate = "2026-01-01"
	if got := DisplayProjectStatusFromOverdueFlag(p, true); got != ProjectStatusCompleted {
		t.Fatalf("expected completed, got %s", got)
	}
}

func TestDisplayProjectStatusWithSubtasks(t *testing.T) {
	p := Project{StartDate: "2020-01-01"}
	subtasks := []ProjectSubtask{
		{PlannedStartDate: "2020-01-01", ActualStartDate: "2020-01-02", PlannedEndDate: "2020-01-10"},
	}
	if got := DisplayProjectStatus(p, subtasks); got != ProjectStatusOverdue {
		t.Fatalf("expected overdue, got %s", got)
	}
}
