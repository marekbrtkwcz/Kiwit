-- +goose Up
CREATE TABLE test_case_datasets (
    id           BIGSERIAL PRIMARY KEY,
    test_case_id BIGINT NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    params       TEXT NOT NULL DEFAULT '{}', -- JSON object of data-driven parameters
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_datasets_case ON test_case_datasets(test_case_id);

ALTER TABLE test_executions ADD COLUMN dataset_id BIGINT REFERENCES test_case_datasets(id) ON DELETE SET NULL;
CREATE INDEX idx_executions_dataset ON test_executions(dataset_id);

-- +goose Down
DROP INDEX IF EXISTS idx_executions_dataset;
ALTER TABLE test_executions DROP COLUMN IF EXISTS dataset_id;
DROP INDEX IF EXISTS idx_datasets_case;
DROP TABLE IF EXISTS test_case_datasets;
