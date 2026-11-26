-- +goose Up
ALTER TABLE visual_comparisons ADD COLUMN IF NOT EXISTS ai_verdict TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE visual_comparisons DROP COLUMN IF EXISTS ai_verdict;
