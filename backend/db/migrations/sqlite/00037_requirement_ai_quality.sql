-- +goose Up
ALTER TABLE requirements ADD COLUMN ai_quality TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite cannot easily drop a column; table would need a rebuild.
