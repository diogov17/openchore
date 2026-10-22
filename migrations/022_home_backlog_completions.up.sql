CREATE TABLE home_backlog_completions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chore_id INTEGER NOT NULL UNIQUE REFERENCES chores(id) ON DELETE CASCADE,
    completed_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    completion_date DATE NOT NULL,
    completed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_home_backlog_completed_by ON home_backlog_completions(completed_by);
