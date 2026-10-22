package api_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)



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
