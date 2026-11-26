-- +goose Up
CREATE TABLE test_plan_types (
    id   BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);
ALTER TABLE test_plans ADD COLUMN plan_type_id BIGINT REFERENCES test_plan_types(id) ON DELETE SET NULL;
CREATE INDEX idx_test_plans_type ON test_plans(plan_type_id);

-- +goose Down
DROP INDEX IF EXISTS idx_test_plans_type;
ALTER TABLE test_plans DROP COLUMN IF EXISTS plan_type_id;
DROP TABLE IF EXISTS test_plan_types;
