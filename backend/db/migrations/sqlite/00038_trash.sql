-- +goose Up
ALTER TABLE test_cases ADD COLUMN deleted_at DATETIME NULL;
CREATE INDEX idx_test_cases_deleted_at ON test_cases(deleted_at) WHERE deleted_at IS NOT NULL;

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35; table would need rebuild.
DROP INDEX IF EXISTS idx_test_cases_deleted_at;
