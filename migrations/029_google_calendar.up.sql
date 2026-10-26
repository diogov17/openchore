CREATE TABLE google_calendar_connections (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    token BLOB NOT NULL,
    calendar_id TEXT NOT NULL DEFAULT 'primary'
);
