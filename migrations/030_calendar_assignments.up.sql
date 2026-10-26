CREATE TABLE google_calendar_chores (
    chore_id INTEGER PRIMARY KEY REFERENCES chores(id) ON DELETE CASCADE,
    timezone TEXT NOT NULL,
    anchor_date TEXT NOT NULL
);

-- Keep event references after a schedule/chore is removed so remote deletion
-- can be retried after a restart. Deliberately no cascading foreign keys.
CREATE TABLE google_calendar_events (
    event_id TEXT PRIMARY KEY,
    chore_id INTEGER NOT NULL,
    schedule_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    calendar_id TEXT NOT NULL,
    payload TEXT NOT NULL DEFAULT ''
);
CREATE INDEX google_calendar_events_chore ON google_calendar_events(chore_id);
