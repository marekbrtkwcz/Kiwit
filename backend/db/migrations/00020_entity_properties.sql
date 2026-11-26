-- +goose Up
CREATE TABLE entity_properties (
    id          BIGSERIAL PRIMARY KEY,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('test_case', 'test_run', 'environment')),
    entity_id   BIGINT NOT NULL,
    name        TEXT NOT NULL,
    value       TEXT NOT NULL DEFAULT '',
    UNIQUE (entity_type, entity_id, name)
);
CREATE INDEX idx_entity_properties_lookup ON entity_properties (entity_type, entity_id);

CREATE TABLE test_run_cc (
    run_id  BIGINT NOT NULL REFERENCES test_runs(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (run_id, user_id)
);
CREATE INDEX idx_test_run_cc_user ON test_run_cc(user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_test_run_cc_user;
DROP TABLE IF EXISTS test_run_cc;
DROP INDEX IF EXISTS idx_entity_properties_lookup;
DROP TABLE IF EXISTS entity_properties;
