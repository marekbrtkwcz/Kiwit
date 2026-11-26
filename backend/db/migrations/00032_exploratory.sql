-- +goose Up
CREATE TABLE exploratory_sessions (
    id               BIGSERIAL PRIMARY KEY,
    product_id       BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    test_run_id      BIGINT REFERENCES test_runs(id) ON DELETE SET NULL,
    charter          TEXT NOT NULL, -- mission / goal of the session
    timebox_minutes  INT NOT NULL DEFAULT 60,
    tester_id        BIGINT REFERENCES users(id) ON DELETE SET NULL,
    notes            TEXT NOT NULL DEFAULT '',
    findings         TEXT NOT NULL DEFAULT '', -- JSON array of finding strings
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_exploratory_product ON exploratory_sessions(product_id);
CREATE INDEX idx_exploratory_run ON exploratory_sessions(test_run_id);

-- +goose Down
DROP TABLE IF EXISTS exploratory_sessions;
