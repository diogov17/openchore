package store

import (
	"context"
	"fmt"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

// nextChoreDate finds the first occurrence after a completion. Weekday rows
// belonging to the same chore/assignee are one repeating chore, not a backlog
// of independent Monday, Tuesday, ... chores.
func nextChoreDate(cs model.ChoreSchedule, after, target string) string {
	lower := normalizeDate(cs.CreatedAt)
	if lower > target {
		lower = target
	}
	if cs.StartDate != nil {
		lower = normalizeDate(*cs.StartDate)
	}
	if cs.RecurrenceStart != nil {
		lower = normalizeDate(*cs.RecurrenceStart)
	}
	if cs.SpecificDate != nil {
		lower = normalizeDate(*cs.SpecificDate)
	}
	if after >= lower {
		d, err := time.Parse(model.DateFormat, after)
		if err != nil {
			return ""
		}
		lower = d.AddDate(0, 0, 1).Format(model.DateFormat)
	}
	if cs.StartDate != nil && lower < normalizeDate(*cs.StartDate) {
		lower = normalizeDate(*cs.StartDate)
	}
	d, err := time.Parse(model.DateFormat, lower)
	if err != nil {
		return ""
	}
	if cs.SpecificDate != nil {
		if lower != normalizeDate(*cs.SpecificDate) {
			return ""
		}
	} else if cs.RecurrenceInterval != nil && cs.RecurrenceStart != nil && *cs.RecurrenceInterval > 0 {
		start, err := time.Parse(model.DateFormat, normalizeDate(*cs.RecurrenceStart))
		if err != nil {
			return ""
		}
		days := int(d.Sub(start).Hours() / 24)
		if rem := days % *cs.RecurrenceInterval; rem != 0 {
			d = d.AddDate(0, 0, *cs.RecurrenceInterval-rem)
		}
	} else if cs.DayOfWeek != nil {
		d = d.AddDate(0, 0, (*cs.DayOfWeek-int(d.Weekday())+7)%7)
	} else {
		return ""
	}
	result := d.Format(model.DateFormat)
	if result > target || (cs.EndDate != nil && result > normalizeDate(*cs.EndDate)) {
		return ""
	}
	return result
}

// Persistent schedules are selected as of the requested date. Completion dates
// remain the day the work was actually done, preserving points and undo.
func (s *Store) persistentScheduleIDs(ctx context.Context, userID int64, date string) (map[int64]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT cs.id, cs.chore_id, cs.assigned_to, cs.assignment_type,
 cs.day_of_week, cs.specific_date, cs.start_date, cs.end_date, cs.recurrence_interval, cs.recurrence_start, cs.created_at,
 COALESCE((SELECT MAX(date(cc.completion_date)) FROM chore_completions cc
 JOIN chore_schedules other ON other.id=cc.chore_schedule_id
 WHERE other.chore_id=cs.chore_id AND other.assignment_type=cs.assignment_type
 AND (other.assigned_to=cs.assigned_to OR cs.assignment_type='family')
 AND cc.uncompleted_at IS NULL AND cc.status IN ('approved','excused') AND date(cc.completion_date) < ?), '')
 FROM chore_schedules cs JOIN chores c ON c.id=cs.chore_id
 WHERE c.reset_daily=0 AND (cs.assigned_to=? OR (cs.assignment_type='family' AND EXISTS(SELECT 1 FROM users WHERE id=? AND role='admin')))
 ORDER BY cs.id`, date, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type choice struct {
		id   int64
		date string
	}
	choices := map[string]choice{}
	for rows.Next() {
		var cs model.ChoreSchedule
		var after string
		if err := rows.Scan(&cs.ID, &cs.ChoreID, &cs.AssignedTo, &cs.AssignmentType, &cs.DayOfWeek, &cs.SpecificDate, &cs.StartDate, &cs.EndDate, &cs.RecurrenceInterval, &cs.RecurrenceStart, &cs.CreatedAt, &after); err != nil {
			return nil, err
		}
		due := nextChoreDate(cs, after, date)
		if due == "" {
			continue
		}
		key := fmt.Sprintf("%d/%d/%s", cs.ChoreID, cs.AssignedTo, cs.AssignmentType)
		if cs.AssignmentType == model.AssignmentFamily {
			key = fmt.Sprintf("%d/family", cs.ChoreID)
		}
		current, ok := choices[key]
		if !ok || due < current.date {
			choices[key] = choice{cs.ID, due}
		}
	}
	ids := map[int64]string{}
	for _, c := range choices {
		ids[c.id] = c.date
	}
	return ids, rows.Err()
}
