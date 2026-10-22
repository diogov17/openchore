UPDATE home_backlog_completions
SET uncompleted_at = superseded_at
WHERE superseded_at IS NOT NULL AND uncompleted_at IS NULL;

DROP INDEX idx_home_backlog_active_chore;
ALTER TABLE home_backlog_completions DROP COLUMN superseded_at;
CREATE UNIQUE INDEX idx_home_backlog_active_chore
    ON home_backlog_completions(chore_id)
    WHERE uncompleted_at IS NULL;
