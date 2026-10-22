package store

import (
	"context"
	"sort"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

func (s *Store) CreateQuickChore(ctx context.Context, c *model.Chore, assignedTo int64, date string, interval int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	category := c.Category
	if category == model.CategoryIndependent || category == model.CategoryHome {
		category = model.CategoryCore
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO chores (title,description,category,is_home,is_independent,reset_daily,points_value,requires_approval,requires_photo,created_by)
 VALUES (?,?,?,?,?,?,?,?,?,?)`, c.Title, c.Description, category, c.Category == model.CategoryHome, c.Category == model.CategoryIndependent, c.ResetDaily, c.PointsValue, c.RequiresApproval, c.RequiresPhoto, c.CreatedBy)
	if err != nil {
		return err
	}
	c.ID, err = result.LastInsertId()
	if err != nil {
		return err
	}
	var specific, recurrence, origin any = date, nil, nil
	if interval > 0 {
		specific = nil
		recurrence = interval
		origin = date
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO chore_schedules (chore_id,assigned_to,specific_date,recurrence_interval,recurrence_start) VALUES (?,?,?,?,?)`, c.ID, assignedTo, specific, recurrence, origin)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CanViewChore(ctx context.Context, id, userID int64, admin bool) (bool, error) {
	var allowed bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM chores c WHERE c.id=? AND (? OR c.created_by=? OR EXISTS(SELECT 1 FROM chore_schedules cs WHERE cs.chore_id=c.id AND cs.assigned_to=?) OR EXISTS(SELECT 1 FROM home_chore_assignments h WHERE h.chore_id=c.id AND h.assigned_to=?)))`, id, admin, userID, userID, userID).Scan(&allowed)
	return allowed, err
}

type ChoreHistoryEntry struct {
	Event  string `json:"event"`
	At     string `json:"at"`
	Person string `json:"person,omitempty"`
	Points int    `json:"points"`
	Date   string `json:"date,omitempty"`
}

func (s *Store) ChoreHistory(ctx context.Context, id, viewer int64, now time.Time) ([]ChoreHistoryEntry, error) {
	result := []ChoreHistoryEntry{}
	rows, err := s.db.QueryContext(ctx, `SELECT 'created',strftime('%Y-%m-%dT%H:%M:%SZ',c.created_at),u.name,0,'' FROM chores c JOIN users u ON u.id=c.created_by WHERE c.id=?
 UNION ALL SELECT cc.status,strftime('%Y-%m-%dT%H:%M:%SZ',cc.completed_at),u.name,
 COALESCE((SELECT SUM(p.amount) FROM point_transactions p WHERE p.reference_id=cc.id AND p.reason IN ('chore_complete','chore_uncomplete')),0),date(cc.completion_date)
 FROM chore_completions cc JOIN chore_schedules cs ON cs.id=cc.chore_schedule_id JOIN users u ON u.id=cc.completed_by
 WHERE cs.chore_id=? AND (?=0 OR cs.assigned_to=?)
 UNION ALL SELECT 'undone',strftime('%Y-%m-%dT%H:%M:%SZ',cc.uncompleted_at),u.name,0,date(cc.completion_date)
 FROM chore_completions cc JOIN chore_schedules cs ON cs.id=cc.chore_schedule_id JOIN users u ON u.id=cc.completed_by
 WHERE cs.chore_id=? AND cc.uncompleted_at IS NOT NULL AND (?=0 OR cs.assigned_to=?)
 UNION ALL SELECT 'approved',strftime('%Y-%m-%dT%H:%M:%SZ',h.completed_at),u.name,h.points_awarded,date(h.completion_date)
 FROM home_backlog_completions h JOIN users u ON u.id=h.completed_by WHERE h.chore_id=? AND (?=0 OR h.completed_by=?)
 UNION ALL SELECT 'undone',strftime('%Y-%m-%dT%H:%M:%SZ',h.uncompleted_at),u.name,0,date(h.completion_date)
 FROM home_backlog_completions h JOIN users u ON u.id=h.completed_by WHERE h.chore_id=? AND h.uncompleted_at IS NOT NULL AND (?=0 OR h.completed_by=?)
 UNION ALL SELECT 'repeated',strftime('%Y-%m-%dT%H:%M:%SZ',h.superseded_at),'',0,'' FROM home_backlog_completions h
 WHERE h.chore_id=? AND h.superseded_at IS NOT NULL AND (?=0 OR h.completed_by=?)`, id, id, viewer, viewer, id, viewer, viewer, id, viewer, viewer, id, viewer, viewer, id, viewer, viewer)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var e ChoreHistoryEntry
		if err := rows.Scan(&e.Event, &e.At, &e.Person, &e.Points, &e.Date); err != nil {
			rows.Close()
			return nil, err
		}
		result = append(result, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	chore, err := s.GetChore(ctx, id)
	if err != nil {
		return nil, err
	}
	schedules, err := s.ListSchedulesForChore(ctx, id)
	if err != nil {
		return nil, err
	}
	availability, err := s.choreAvailabilityHistory(ctx, chore, schedules, viewer, now)
	if err != nil {
		return nil, err
	}
	result = append(result, availability...)
	sort.SliceStable(result, func(i, j int) bool { return result[i].At > result[j].At })
	return result, nil
}
