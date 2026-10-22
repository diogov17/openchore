ALTER TABLE home_backlog_completions ADD COLUMN superseded_at DATETIME;

DROP INDEX idx_home_backlog_active_chore;
CREATE UNIQUE INDEX idx_home_backlog_active_chore
    ON home_backlog_completions(chore_id)
    WHERE uncompleted_at IS NULL AND superseded_at IS NULL;
