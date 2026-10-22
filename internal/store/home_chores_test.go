package store_test

import (
	"context"
	"testing"

	"github.com/liftedkilt/openchore/internal/model"
)

func TestHomeBacklogAssignmentRepeatAndUndo(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, "Alex", "admin")
	c := createTestChore(t, s, "Clean windows", 5, u.ID)
	c.Category = model.CategoryHome
	if err := s.UpdateChore(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err := s.AssignHomeBacklogChore(ctx, c.ID, u.ID, false); err != nil {
		t.Fatal(err)
	}
	assigned, err := s.ListAssignedHomeChores(ctx, u.ID, "2026-10-22")
	if err != nil || len(assigned) != 1 {
		t.Fatalf("assignment: %+v, %v", assigned, err)
	}
	if err := s.CompleteAssignedHomeBacklogChore(ctx, c.ID, u.ID, "2026-10-22"); err != nil {
		t.Fatal(err)
	}
	if points, _ := s.GetPointBalance(ctx, u.ID); points != 5 {
		t.Fatalf("completion points=%d", points)
	}
	if err := s.RepeatHomeBacklogChore(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	history, err := s.ListHomeBacklogHistory(ctx)
	if err != nil || len(history) != 1 {
		t.Fatalf("repeat history: %+v, %v", history, err)
	}
	if err := s.CompleteAssignedHomeBacklogChore(ctx, c.ID, u.ID, "2026-10-23"); err != nil {
		t.Fatal(err)
	}
	if err := s.UncompleteAssignedHomeBacklogChore(ctx, c.ID, u.ID); err != nil {
		t.Fatal(err)
	}
	if points, _ := s.GetPointBalance(ctx, u.ID); points != 5 {
		t.Fatalf("undo should preserve the earlier round's points, got %d", points)
	}
	assigned, err = s.ListAssignedHomeChores(ctx, u.ID, "2026-10-23")
	if err != nil || len(assigned) != 1 || assigned[0].CompletedAt != nil {
		t.Fatalf("undo did not reopen: %+v, %v", assigned, err)
	}
}
