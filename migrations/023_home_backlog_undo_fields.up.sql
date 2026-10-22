ALTER TABLE home_backlog_completions RENAME TO home_backlog_completions_v22;
DROP INDEX IF EXISTS idx_home_backlog_completed_by;

CREATE TABLE home_backlog_completions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chore_id INTEGER NOT NULL REFERENCES chores(id) ON DELETE CASCADE,
    completed_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    points_awarded INTEGER NOT NULL DEFAULT 0,
    point_reference_id INTEGER NOT NULL DEFAULT 0,
    completion_date DATE NOT NULL,
    completed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    uncompleted_at DATETIME
);

INSERT INTO home_backlog_completions (id, chore_id, completed_by, points_awarded, point_reference_id, completion_date, completed_at)
SELECT old.id, old.chore_id, old.completed_by,
       COALESCE((
           SELECT SUM(pt.amount) FROM point_transactions pt
           WHERE pt.reference_id IN (old.id, -old.id)
             AND pt.reason = 'chore_complete'
             AND pt.note = 'Home backlog chore'
       ), 0),
       CASE WHEN EXISTS (
           SELECT 1 FROM point_transactions pt
           WHERE pt.reference_id = -old.id
             AND pt.reason = 'chore_complete'
             AND pt.note = 'Home backlog chore'
       ) THEN -old.id ELSE old.id END,
       old.completion_date, old.completed_at
FROM home_backlog_completions_v22 old;

DROP TABLE home_backlog_completions_v22;
CREATE INDEX idx_home_backlog_completed_by ON home_backlog_completions(completed_by);
CREATE UNIQUE INDEX idx_home_backlog_active_chore ON home_backlog_completions(chore_id) WHERE uncompleted_at IS NULL;
