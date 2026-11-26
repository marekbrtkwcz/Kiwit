-- +goose Up
CREATE TABLE milestones (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    version_id   INTEGER NOT NULL REFERENCES versions(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    due_date     TEXT,
    status       TEXT NOT NULL DEFAULT 'planned',
    signoff_decision TEXT,
    signoff_user_id  INTEGER REFERENCES users(id) ON DELETE SET NULL,
    signoff_note     TEXT NOT NULL DEFAULT '',
    signoff_at       DATETIME,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_milestones_version ON milestones(version_id);

CREATE TABLE milestone_runs (
    milestone_id INTEGER NOT NULL REFERENCES milestones(id) ON DELETE CASCADE,
    test_run_id  INTEGER NOT NULL REFERENCES test_runs(id) ON DELETE CASCADE,
    PRIMARY KEY (milestone_id, test_run_id)
);
CREATE INDEX idx_milestone_runs_run ON milestone_runs(test_run_id);

-- +goose Down
DROP TABLE IF EXISTS milestone_runs;
DROP INDEX IF EXISTS idx_milestones_version;
DROP TABLE IF EXISTS milestones;
