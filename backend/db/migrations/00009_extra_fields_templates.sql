-- +goose Up
ALTER TABLE test_cases ADD COLUMN is_automated BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE test_cases ADD COLUMN script TEXT NOT NULL DEFAULT '';
ALTER TABLE test_cases ADD COLUMN arguments TEXT NOT NULL DEFAULT '';

CREATE TABLE templates (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    body       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS templates;
ALTER TABLE test_cases DROP COLUMN IF EXISTS arguments;
ALTER TABLE test_cases DROP COLUMN IF EXISTS script;
ALTER TABLE test_cases DROP COLUMN IF EXISTS is_automated;
