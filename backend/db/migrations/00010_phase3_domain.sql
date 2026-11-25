-- +goose Up
ALTER TABLE test_cases ADD COLUMN default_tester_id BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE test_cases ADD COLUMN reviewer_id BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE test_cases ADD COLUMN requirement TEXT NOT NULL DEFAULT '';
ALTER TABLE test_cases ADD COLUMN extra_link TEXT NOT NULL DEFAULT '';
ALTER TABLE test_cases ADD COLUMN notes TEXT NOT NULL DEFAULT '';

ALTER TABLE test_plans ADD COLUMN plan_type TEXT NOT NULL DEFAULT 'regression';
ALTER TABLE test_plans ADD COLUMN extra_link TEXT NOT NULL DEFAULT '';

ALTER TABLE test_runs ADD COLUMN environment_id BIGINT REFERENCES environments(id) ON DELETE SET NULL;
ALTER TABLE test_runs ADD COLUMN default_tester_id BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE test_runs ADD COLUMN planned_start TIMESTAMPTZ;
ALTER TABLE test_runs ADD COLUMN planned_stop TIMESTAMPTZ;

ALTER TABLE test_executions ADD COLUMN environment_id BIGINT REFERENCES environments(id) ON DELETE SET NULL;
ALTER TABLE test_executions ADD COLUMN case_text_version INT NOT NULL DEFAULT 1;
ALTER TABLE test_executions ADD COLUMN started_at TIMESTAMPTZ;
ALTER TABLE test_executions ADD COLUMN stopped_at TIMESTAMPTZ;

CREATE TABLE execution_links (
    id                BIGSERIAL PRIMARY KEY,
    test_execution_id BIGINT NOT NULL REFERENCES test_executions(id) ON DELETE CASCADE,
    name              TEXT NOT NULL,
    url               TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_execution_links_exec ON execution_links(test_execution_id);

-- +goose Down
DROP TABLE IF EXISTS execution_links;
ALTER TABLE test_executions DROP COLUMN IF EXISTS stopped_at;
ALTER TABLE test_executions DROP COLUMN IF EXISTS started_at;
ALTER TABLE test_executions DROP COLUMN IF EXISTS case_text_version;
ALTER TABLE test_executions DROP COLUMN IF EXISTS environment_id;
ALTER TABLE test_runs DROP COLUMN IF EXISTS planned_stop;
ALTER TABLE test_runs DROP COLUMN IF EXISTS planned_start;
ALTER TABLE test_runs DROP COLUMN IF EXISTS default_tester_id;
ALTER TABLE test_runs DROP COLUMN IF EXISTS environment_id;
ALTER TABLE test_plans DROP COLUMN IF EXISTS extra_link;
ALTER TABLE test_plans DROP COLUMN IF EXISTS plan_type;
ALTER TABLE test_cases DROP COLUMN IF EXISTS notes;
ALTER TABLE test_cases DROP COLUMN IF EXISTS extra_link;
ALTER TABLE test_cases DROP COLUMN IF EXISTS requirement;
ALTER TABLE test_cases DROP COLUMN IF EXISTS reviewer_id;
ALTER TABLE test_cases DROP COLUMN IF EXISTS default_tester_id;
