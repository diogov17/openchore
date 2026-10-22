package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

func TestPersistentChoreCarryCompleteUndoAndHistory(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, "Alex", "admin")
	c := &model.Chore{Title: "Fix the shelf", Category: model.CategoryIndependent, PointsValue: 5, CreatedBy: u.ID}
	if err := s.CreateQuickChore(ctx, c, u.ID, "2026-10-19", 0); err != nil {
		t.Fatal(err)
	}
	query := func(date string) []model.ScheduledChore {
		t.Helper()
		now, _ := time.Parse(model.DateFormat, date)
		rows, err := s.GetScheduledChoresForUser(ctx, u.ID, []string{date}, now)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	if rows := query("2026-10-18"); len(rows) != 0 {
		t.Fatal("chore appeared before start")
	}
	rows := query("2026-10-22")
	if len(rows) != 1 || rows[0].Completed || rows[0].Category != model.CategoryIndependent || rows[0].ScheduledFor != "2026-10-19" {
		t.Fatalf("carry: %+v", rows)
	}
	cc := &model.ChoreCompletion{ChoreScheduleID: rows[0].ScheduleID, CompletedBy: u.ID, Status: model.StatusApproved, CompletionDate: "2026-10-22"}
	if err := s.CompleteChoreAndCreditPoints(ctx, cc, 5, 0); err != nil {
		t.Fatal(err)
	}
	if rows = query("2026-10-22"); len(rows) != 1 || !rows[0].Completed {
		t.Fatalf("done today: %+v", rows)
	}
	if rows = query("2026-10-23"); len(rows) != 0 {
		t.Fatalf("completed chore returned tomorrow: %+v", rows)
	}
	if err := s.UncompleteChoreAndDebitPoints(ctx, cc.ChoreScheduleID, cc.CompletionDate, u.ID, cc.ID, 5); err != nil {
		t.Fatal(err)
	}
	if rows = query("2026-10-23"); len(rows) != 1 || rows[0].Completed {
		t.Fatalf("undo did not reopen: %+v", rows)
	}
	history, err := s.ChoreHistory(ctx, c.ID, 0, time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	events := map[string]bool{}
	for _, e := range history {
		events[e.Event] = true
	}
	for _, event := range []string{"created", "available", "approved", "undone"} {
		if !events[event] {
			t.Errorf("history missing %s", event)
		}
	}
}

func TestPersistentWeekdaySchedulesAreOneTask(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, "Alex", "admin")
	c := &model.Chore{Title: "Clean", Category: model.CategoryCore, CreatedBy: u.ID}
	if err := s.CreateChore(ctx, c); err != nil {
		t.Fatal(err)
	}
	start := "2026-10-19"
	for _, day := range []int{1, 3, 5} {
		d := day
		if err := s.CreateSchedule(ctx, &model.ChoreSchedule{ChoreID: c.ID, AssignedTo: u.ID, AssignmentType: "individual", DayOfWeek: &d, StartDate: &start, PointsMultiplier: 1}); err != nil {
			t.Fatal(err)
		}
	}
	query := func(date string) []model.ScheduledChore {
		t.Helper()
		now, _ := time.Parse(model.DateFormat, date)
		rows, err := s.GetScheduledChoresForUser(ctx, u.ID, []string{date}, now)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	rows := query("2026-10-22")
	if len(rows) != 1 {
		t.Fatalf("duplicate weekday chores: %+v", rows)
	}
	if err := s.CompleteChore(ctx, &model.ChoreCompletion{ChoreScheduleID: rows[0].ScheduleID, CompletedBy: u.ID, Status: "approved", CompletionDate: "2026-10-22"}); err != nil {
		t.Fatal(err)
	}
	rows = query("2026-10-23")
	if len(rows) != 1 || rows[0].ScheduledFor != "2026-10-23" {
		t.Fatalf("next repetition: %+v", rows)
	}
}

func TestPendingPersistentChoreSurvivesMidnight(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, "Alex", "child")
	c := &model.Chore{Title: "Clean", Category: model.CategoryIndependent, CreatedBy: u.ID, RequiresApproval: true}
	if err := s.CreateQuickChore(ctx, c, u.ID, "2026-10-19", 1); err != nil {
		t.Fatal(err)
	}
	schedules, err := s.ListSchedulesForChore(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteChore(ctx, &model.ChoreCompletion{ChoreScheduleID: schedules[0].ID, CompletedBy: u.ID, Status: "pending", CompletionDate: "2026-10-20"}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.GetScheduledChoresForUser(ctx, u.ID, []string{"2026-10-22"}, time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !rows[0].Completed || rows[0].CompletionDate != "2026-10-20" || *rows[0].CompletionStatus != "pending" {
		t.Fatalf("pending chore was lost or duplicated: %+v", rows)
	}
}

func TestResetChoreStartsFreshAndRecordsMissed(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, "Alex", "admin")
	c := &model.Chore{Title: "Daily practice", Category: model.CategoryCore, CreatedBy: u.ID, ResetDaily: true}
	if err := s.CreateQuickChore(ctx, c, u.ID, "2026-10-19", 2); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	rows, err := s.GetScheduledChoresForUser(ctx, u.ID, []string{"2026-10-22"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatal("reset chore carried into an unscheduled day")
	}
	rows, err = s.GetScheduledChoresForUser(ctx, u.ID, []string{"2026-10-23"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Completed {
		t.Fatal("fresh occurrence missing")
	}
	history, err := s.ChoreHistory(ctx, c.ID, 0, now)
	if err != nil {
		t.Fatal(err)
	}
	missed := 0
	for _, e := range history {
		if e.Event == "missed" {
			missed++
		}
	}
	if missed != 2 {
		t.Fatalf("wanted two missed scheduled days, got %d", missed)
	}
}
