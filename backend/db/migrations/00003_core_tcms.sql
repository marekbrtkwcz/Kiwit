-- +goose Up
CREATE TABLE test_cases (
    id            BIGSERIAL PRIMARY KEY,
    product_id    BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    summary       TEXT NOT NULL,
    preconditions TEXT NOT NULL DEFAULT '',
    format        TEXT NOT NULL DEFAULT 'steps', -- 'steps' | 'gherkin'
    gherkin_body  TEXT NOT NULL DEFAULT '',
    priority      TEXT NOT NULL DEFAULT 'medium', -- 'low' | 'medium' | 'high' | 'critical'
    status        TEXT NOT NULL DEFAULT 'confirmed', -- 'draft' | 'confirmed' | 'disabled'
    author_id     BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE test_case_steps (
    id              BIGSERIAL PRIMARY KEY,
    test_case_id    BIGINT NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
    position        INT NOT NULL,
    action          TEXT NOT NULL,
    expected_result TEXT NOT NULL DEFAULT '',
    UNIQUE (test_case_id, position)
);

CREATE TABLE test_plans (
    id          BIGSERIAL PRIMARY KEY,
    product_id  BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    version_id  BIGINT NOT NULL REFERENCES versions(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    author_id   BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE test_plan_cases (
    test_plan_id BIGINT NOT NULL REFERENCES test_plans(id) ON DELETE CASCADE,
    test_case_id BIGINT NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
    position     INT NOT NULL DEFAULT 0,
    PRIMARY KEY (test_plan_id, test_case_id)
);

CREATE TABLE test_runs (
    id           BIGSERIAL PRIMARY KEY,
    test_plan_id BIGINT NOT NULL REFERENCES test_plans(id) ON DELETE CASCADE,
    build_id     BIGINT NOT NULL REFERENCES builds(id) ON DELETE RESTRICT,
    summary      TEXT NOT NULL,
    notes        TEXT NOT NULL DEFAULT '',
    manager_id   BIGINT REFERENCES users(id) ON DELETE SET NULL,
    started_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    stopped_at   TIMESTAMPTZ
);

CREATE TABLE test_executions (
    id             BIGSERIAL PRIMARY KEY,
    test_run_id    BIGINT NOT NULL REFERENCES test_runs(id) ON DELETE CASCADE,
    test_case_id   BIGINT NOT NULL REFERENCES test_cases(id) ON DELETE RESTRICT,
    status         TEXT NOT NULL DEFAULT 'idle', -- 'idle' | 'running' | 'passed' | 'failed' | 'blocked' | 'skipped'
    assignee_id    BIGINT REFERENCES users(id) ON DELETE SET NULL,
    tested_by_id   BIGINT REFERENCES users(id) ON DELETE SET NULL,
    steps_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,
    comment        TEXT NOT NULL DEFAULT '',
    executed_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS test_executions;
DROP TABLE IF EXISTS test_runs;
DROP TABLE IF EXISTS test_plan_cases;
DROP TABLE IF EXISTS test_plans;
DROP TABLE IF EXISTS test_case_steps;
DROP TABLE IF EXISTS test_cases;
