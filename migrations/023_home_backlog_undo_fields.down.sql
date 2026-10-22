DROP INDEX IF EXISTS idx_home_backlog_active_chore;
DELETE FROM home_backlog_completions
WHERE id NOT IN (
    SELECT MAX(id) FROM home_backlog_completions
    WHERE uncompleted_at IS NULL
    GROUP BY chore_id
);
ALTER TABLE home_backlog_completions DROP COLUMN uncompleted_at;
ALTER TABLE home_backlog_completions DROP COLUMN point_reference_id;
ALTER TABLE home_backlog_completions DROP COLUMN points_awarded;
CREATE UNIQUE INDEX idx_home_backlog_active_chore ON home_backlog_completions(chore_id);
