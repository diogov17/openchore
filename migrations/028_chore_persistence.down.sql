DROP INDEX IF EXISTS idx_completions_active_date;
ALTER TABLE chores DROP COLUMN is_independent;
ALTER TABLE chores DROP COLUMN reset_daily;
