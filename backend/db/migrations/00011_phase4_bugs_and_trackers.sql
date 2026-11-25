-- +goose Up
ALTER TABLE issue_trackers ADD COLUMN api_username TEXT NOT NULL DEFAULT '';
ALTER TABLE bugs ADD COLUMN last_synced_at TIMESTAMPTZ;

CREATE TABLE internal_bugs (
    id                BIGSERIAL PRIMARY KEY,
    product_id        BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    build_id          BIGINT REFERENCES builds(id) ON DELETE SET NULL,
    test_execution_id BIGINT REFERENCES test_executions(id) ON DELETE SET NULL,
    summary           TEXT NOT NULL,
    description       TEXT NOT NULL DEFAULT '',
    status            TEXT NOT NULL DEFAULT 'open',
    severity          TEXT NOT NULL DEFAULT 'medium',
    reporter_id       BIGINT REFERENCES users(id) ON DELETE SET NULL,
    assignee_id       BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_internal_bugs_product ON internal_bugs(product_id);
CREATE INDEX idx_internal_bugs_exec ON internal_bugs(test_execution_id);

-- +goose Down
DROP TABLE IF EXISTS internal_bugs;
ALTER TABLE bugs DROP COLUMN IF EXISTS last_synced_at;
ALTER TABLE issue_trackers DROP COLUMN IF EXISTS api_username;
