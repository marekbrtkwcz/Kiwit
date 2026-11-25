-- +goose Up
ALTER TABLE test_plans ADD COLUMN parent_id BIGINT REFERENCES test_plans(id) ON DELETE SET NULL;

CREATE TABLE environments (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    properties  JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS environments;
ALTER TABLE test_plans DROP COLUMN IF EXISTS parent_id;
