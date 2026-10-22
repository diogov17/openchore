package api_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

func TestWeekBackfillUsesOriginalDate(t *testing.T) {
	for _, reset := range []bool{false, true} {
		t.Run(fmt.Sprintf("reset=%t", reset), func(t *testing.T) {
			env := setupTest(t)
			env.createAdmin(t)
			kid := env.createChild(t, "Alex")
			date := time.Now().AddDate(0, 0, -1).Format(model.DateFormat)
			resp := env.expectStatus(t, "POST", "/api/chores/quick", map[string]any{"title": "Forgot to check", "date": date, "assigned_to": kid, "points_value": 5, "reset_daily": reset}, adminHeaders(), 201)
			var chore model.Chore
			decodeBody(t, resp, &chore)
			schedules, err := env.store.ListSchedulesForChore(context.Background(), chore.ID)
			if err != nil {
				t.Fatal(err)
			}
			schedule := schedules[0]
			opens, due := "23:59", "00:01"
			schedule.AvailableAt, schedule.DueBy = &opens, &due
			schedule.ExpiryPenalty = model.ExpiryBlock
			if err := env.store.UpdateSchedule(context.Background(), &schedule); err != nil {
				t.Fatal(err)
			}
			path := fmt.Sprintf("/api/schedules/%d/complete", schedule.ID)
			resp = env.expectStatus(t, "POST", path, map[string]any{"completion_date": date}, childHeaders(kid), 201)
			var completion model.ChoreCompletion
			decodeBody(t, resp, &completion)
			if completion.CompletionDate != date {
				t.Fatalf("backfill changed date: %+v", completion)
			}
			if points := env.balance(t, kid); points != 5 {
				t.Fatalf("wanted 5 points, got %d", points)
			}
			resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/chores?view=weekly&date=%s", kid, date), nil, childHeaders(kid), 200)
			var week []model.ScheduledChore
			decodeBody(t, resp, &week)
			found := false
			for _, c := range week {
				if c.ChoreID == chore.ID && c.Date == date && c.Completed {
					found = true
				}
			}
			if !found {
				t.Fatalf("backfill not completed on original day: %+v", week)
			}
			env.expectStatus(t, "DELETE", path+"?date="+date, nil, childHeaders(kid), 204).Body.Close()
			if points := env.balance(t, kid); points != 0 {
				t.Fatalf("undo left %d points", points)
			}
		})
	}
}

func TestQuickChoreDefaultsAndPermissions(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Alex")
	other := env.createChild(t, "Sam")
	resp := env.expectStatus(t, "POST", "/api/chores/quick", map[string]any{"title": "  Read a chapter  "}, childHeaders(kid), 201)
	var chore model.Chore
	decodeBody(t, resp, &chore)
	if chore.Title != "Read a chapter" || chore.Category != model.CategoryIndependent || chore.PointsValue != 0 || chore.ResetDaily {
		t.Fatalf("bad defaults: %+v", chore)
	}
	resp = env.expectStatus(t, "GET", fmt.Sprintf("/api/users/%d/chores?date=%s", kid, time.Now().Format(model.DateFormat)), nil, childHeaders(kid), 200)
	var rows []model.ScheduledChore
	decodeBody(t, resp, &rows)
	if len(rows) != 1 || rows[0].ChoreID != chore.ID {
		t.Fatalf("chore not assigned to creator: %+v", rows)
	}
	env.expectStatus(t, "POST", "/api/chores/quick", map[string]any{"title": "Cheat", "points_value": 100}, childHeaders(kid), 403).Body.Close()
	env.expectStatus(t, "POST", "/api/chores/quick", map[string]any{"title": "Other", "assigned_to": other}, childHeaders(kid), 403).Body.Close()
	env.expectStatus(t, "GET", fmt.Sprintf("/api/chores/%d/history", chore.ID), nil, childHeaders(other), 404).Body.Close()
	env.expectStatus(t, "GET", fmt.Sprintf("/api/chores/%d/history", chore.ID), nil, childHeaders(kid), 200).Body.Close()
	env.expectStatus(t, "POST", "/api/chores/quick", map[string]any{"title": " "}, adminHeaders(), 400).Body.Close()
	env.expectStatus(t, "POST", "/api/chores/quick", map[string]any{"title": "No identity"}, nil, 401).Body.Close()
}

func TestQuickChoreCompletionUndoAndHistory(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	today := time.Now().Format(model.DateFormat)
	yesterday := time.Now().AddDate(0, 0, -1).Format(model.DateFormat)
	resp := env.expectStatus(t, "POST", "/api/chores/quick", map[string]any{"title": "Fix shelf", "date": yesterday, "points_value": 5}, adminHeaders(), 201)
	var chore model.Chore
	decodeBody(t, resp, &chore)
	resp = env.expectStatus(t, "GET", "/api/users/1/chores?date="+today, nil, adminHeaders(), 200)
	var rows []model.ScheduledChore
	decodeBody(t, resp, &rows)
	if len(rows) != 1 {
		t.Fatalf("carry missing: %+v", rows)
	}
	path := fmt.Sprintf("/api/schedules/%d/complete", rows[0].ScheduleID)
	env.expectStatus(t, "POST", path, map[string]any{"completion_date": today}, adminHeaders(), 201).Body.Close()
	if points := env.balance(t, 1); points != 5 {
		t.Fatalf("wanted 5 points, got %d", points)
	}
	env.expectStatus(t, "DELETE", path+"?date="+today, nil, adminHeaders(), 204).Body.Close()
	if points := env.balance(t, 1); points != 0 {
		t.Fatalf("undo left %d points", points)
	}
	env.expectStatus(t, "GET", fmt.Sprintf("/api/chores/%d/history", chore.ID), nil, adminHeaders(), 200).Body.Close()
}
