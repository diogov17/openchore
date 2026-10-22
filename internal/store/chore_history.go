package store

import (
	"context"
	"fmt"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

func choreGroup(cs model.ChoreSchedule) string {
	if cs.AssignmentType == model.AssignmentFamily {
		return "family"
	}
	return fmt.Sprintf("%d/%s", cs.AssignedTo, cs.AssignmentType)
}

func (s *Store) choreAvailabilityHistory(ctx context.Context, chore *model.Chore, schedules []model.ChoreSchedule, viewer int64, now time.Time) ([]ChoreHistoryEntry, error) {
	result := []ChoreHistoryEntry{}
	groups := map[string][]model.ChoreSchedule{}
	scheduleGroups := map[int64]string{}
	for _, cs := range schedules {
		if viewer != 0 && cs.AssignedTo != viewer {
			continue
		}
		key := choreGroup(cs)
		groups[key] = append(groups[key], cs)
		scheduleGroups[cs.ID] = key
	}
	type completion struct {
		date, status string
		undone       bool
	}
	completions := map[string][]completion{}
	rows, err := s.db.QueryContext(ctx, `SELECT cc.chore_schedule_id,date(cc.completion_date),cc.status,cc.uncompleted_at IS NOT NULL FROM chore_completions cc JOIN chore_schedules cs ON cs.id=cc.chore_schedule_id WHERE cs.chore_id=? ORDER BY cc.completion_date,cc.id`, chore.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var c completion
		if err := rows.Scan(&id, &c.date, &c.status, &c.undone); err != nil {
			rows.Close()
			return nil, err
		}
		if key, ok := scheduleGroups[id]; ok {
			completions[key] = append(completions[key], c)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	today := now.Format(model.DateFormat)
	for key, list := range groups {
		person := ""
		if list[0].AssignmentType != model.AssignmentFamily {
			u, err := s.GetUser(ctx, list[0].AssignedTo)
			if err != nil {
				return nil, err
			}
			if u != nil {
				person = u.Name
			}
		}
		seen := map[string]bool{}
		available := func(date string, event string) {
			if date != "" && !seen[event+date] {
				seen[event+date] = true
				result = append(result, ChoreHistoryEntry{Event: event, At: date + "T00:00:00Z", Date: date, Person: person})
			}
		}
		next := func(after, target string) string {
			due := ""
			for _, cs := range list {
				candidate := nextChoreDate(cs, after, target)
				if candidate != "" && (due == "" || candidate < due) {
					due = candidate
				}
			}
			return due
		}
		if !chore.ResetDaily {
			after := ""
			pending := false
			for _, c := range completions[key] {
				available(next(after, c.date), "available")
				if !c.undone && (c.status == model.StatusApproved || c.status == model.StatusExcused) {
					after = c.date
				}
				if !c.undone && c.status == model.StatusPending {
					pending = true
				}
			}
			if !pending {
				available(next(after, today), "available")
			}
			continue
		}
		for _, cs := range list {
			after := ""
			for {
				due := nextChoreDate(cs, after, today)
				if due == "" {
					break
				}
				after = due
				var exists bool
				if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM chore_completions WHERE chore_schedule_id=? AND date(completion_date)=? AND uncompleted_at IS NULL AND status!='rejected')`, cs.ID, due).Scan(&exists); err != nil {
					return nil, err
				}
				available(due, "available")
				if !exists && due < today {
					available(due, "missed")
				}
			}
		}
	}
	return result, nil
}
