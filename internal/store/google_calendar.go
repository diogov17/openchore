package store

import (
	"context"
	"database/sql"
	"errors"
)

type CalendarConnection struct {
	UserID     int64
	Token      []byte
	CalendarID string
}

func (s *Store) CalendarConnection(ctx context.Context, userID int64) (*CalendarConnection, error) {
	c := &CalendarConnection{UserID: userID}
	err := s.db.QueryRowContext(ctx, `SELECT token, calendar_id FROM google_calendar_connections WHERE user_id=?`, userID).Scan(&c.Token, &c.CalendarID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

func (s *Store) SaveCalendarConnection(ctx context.Context, c *CalendarConnection) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO google_calendar_connections(user_id,token,calendar_id) VALUES(?,?,?) ON CONFLICT(user_id) DO UPDATE SET token=excluded.token,calendar_id=excluded.calendar_id`, c.UserID, c.Token, c.CalendarID)
	return err
}

func (s *Store) DeleteCalendarConnection(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM google_calendar_connections WHERE user_id=?`, userID)
	return err
}

type CalendarChore struct {
	ChoreID    int64
	Timezone   string
	AnchorDate string
}

type CalendarEvent struct {
	EventID                     string
	ChoreID, ScheduleID, UserID int64
	CalendarID, Payload         string
}

func (s *Store) EnableChoreCalendar(ctx context.Context, c CalendarChore) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO google_calendar_chores(chore_id,timezone,anchor_date) VALUES(?,?,?) ON CONFLICT(chore_id) DO NOTHING`, c.ChoreID, c.Timezone, c.AnchorDate)
	return err
}

func (s *Store) ChoreCalendar(ctx context.Context, id int64) (*CalendarChore, error) {
	c := &CalendarChore{ChoreID: id}
	err := s.db.QueryRowContext(ctx, `SELECT timezone,anchor_date FROM google_calendar_chores WHERE chore_id=?`, id).Scan(&c.Timezone, &c.AnchorDate)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

func (s *Store) CalendarChoreIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT chore_id FROM google_calendar_chores UNION SELECT chore_id FROM google_calendar_events`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) ChoreCalendarEvents(ctx context.Context, id int64) ([]CalendarEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT event_id,chore_id,schedule_id,user_id,calendar_id,payload FROM google_calendar_events WHERE chore_id=? ORDER BY event_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []CalendarEvent{}
	for rows.Next() {
		var e CalendarEvent
		if err = rows.Scan(&e.EventID, &e.ChoreID, &e.ScheduleID, &e.UserID, &e.CalendarID, &e.Payload); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *Store) SaveCalendarEvent(ctx context.Context, e CalendarEvent) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO google_calendar_events(event_id,chore_id,schedule_id,user_id,calendar_id,payload) VALUES(?,?,?,?,?,?) ON CONFLICT(event_id) DO UPDATE SET payload=excluded.payload`, e.EventID, e.ChoreID, e.ScheduleID, e.UserID, e.CalendarID, e.Payload)
	return err
}

func (s *Store) DeleteCalendarEvent(ctx context.Context, eventID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM google_calendar_events WHERE event_id=?`, eventID)
	return err
}
