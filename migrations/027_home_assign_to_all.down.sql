ALTER TABLE home_chore_assignments RENAME TO home_chore_assignments_new;
DROP INDEX IF EXISTS idx_home_chore_assignments_active;
DROP INDEX IF EXISTS idx_home_chore_assignments_user;

CREATE TABLE home_chore_assignments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chore_id INTEGER NOT NULL REFERENCES chores(id) ON DELETE CASCADE,
    assigned_to INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assigned_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    unassigned_at DATETIME
);

INSERT INTO home_chore_assignments (id, chore_id, assigned_to, assigned_at, unassigned_at)
SELECT id, chore_id, assigned_to, assigned_at, unassigned_at
FROM home_chore_assignments_new
WHERE assigned_to IS NOT NULL;

DROP TABLE home_chore_assignments_new;

CREATE UNIQUE INDEX idx_home_chore_assignments_active
    ON home_chore_assignments(chore_id)
    WHERE unassigned_at IS NULL;

CREATE INDEX idx_home_chore_assignments_user
    ON home_chore_assignments(assigned_to, unassigned_at);
