ALTER TABLE chores ADD COLUMN reset_daily INTEGER NOT NULL DEFAULT 0;
ALTER TABLE chores ADD COLUMN is_independent INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_completions_active_date ON chore_completions(chore_schedule_id, completion_date, uncompleted_at);
