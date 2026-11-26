-- +goose Up
ALTER TABLE requirements ADD COLUMN IF NOT EXISTS ai_quality TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE requirements DROP COLUMN IF EXISTS ai_quality;
