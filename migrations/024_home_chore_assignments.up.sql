CREATE TABLE home_chore_assignments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chore_id INTEGER NOT NULL REFERENCES chores(id) ON DELETE CASCADE,
    assigned_to INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assigned_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    unassigned_at DATETIME
);

CREATE UNIQUE INDEX idx_home_chore_assignments_active
    ON home_chore_assignments(chore_id)
    WHERE unassigned_at IS NULL;

CREATE INDEX idx_home_chore_assignments_user
    ON home_chore_assignments(assigned_to, unassigned_at);
