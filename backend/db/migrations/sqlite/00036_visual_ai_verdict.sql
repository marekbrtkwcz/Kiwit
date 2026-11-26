-- +goose Up
ALTER TABLE visual_comparisons ADD COLUMN ai_verdict TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35; table would need rebuild.
