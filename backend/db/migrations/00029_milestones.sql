-- +goose Up
CREATE TABLE milestones (
    id           BIGSERIAL PRIMARY KEY,
    version_id   BIGINT NOT NULL REFERENCES versions(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    due_date     TEXT,
    status       TEXT NOT NULL DEFAULT 'planned', -- 'planned' | 'in_progress' | 'done'
    signoff_decision TEXT, -- 'go' | 'no-go' | NULL
    signoff_user_id  BIGINT REFERENCES users(id) ON DELETE SET NULL,
    signoff_note     TEXT NOT NULL DEFAULT '',
    signoff_at       TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_milestones_version ON milestones(version_id);

CREATE TABLE milestone_runs (
    milestone_id BIGINT NOT NULL REFERENCES milestones(id) ON DELETE CASCADE,
    test_run_id  BIGINT NOT NULL REFERENCES test_runs(id) ON DELETE CASCADE,
    PRIMARY KEY (milestone_id, test_run_id)
);
CREATE INDEX idx_milestone_runs_run ON milestone_runs(test_run_id);

-- +goose Down
DROP TABLE IF EXISTS milestone_runs;
DROP INDEX IF EXISTS idx_milestones_version;
DROP TABLE IF EXISTS milestones;
