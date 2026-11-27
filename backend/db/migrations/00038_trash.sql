-- +goose Up
ALTER TABLE test_cases ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX idx_test_cases_deleted_at ON test_cases(deleted_at) WHERE deleted_at IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_test_cases_deleted_at;
ALTER TABLE test_cases DROP COLUMN IF EXISTS deleted_at;
