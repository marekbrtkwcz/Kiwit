-- +goose Up
CREATE TABLE test_case_tags (
    test_case_id INTEGER NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
    tag_id       INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (test_case_id, tag_id)
);

CREATE TABLE test_plan_tags (
    test_plan_id INTEGER NOT NULL REFERENCES test_plans(id) ON DELETE CASCADE,
    tag_id       INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (test_plan_id, tag_id)
);

CREATE TABLE test_run_tags (
    test_run_id INTEGER NOT NULL REFERENCES test_runs(id) ON DELETE CASCADE,
    tag_id      INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (test_run_id, tag_id)
);

CREATE TABLE test_case_components (
    test_case_id INTEGER NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
    component_id INTEGER NOT NULL REFERENCES components(id) ON DELETE CASCADE,
    PRIMARY KEY (test_case_id, component_id)
);

-- +goose Down
DROP TABLE IF EXISTS test_case_components;
DROP TABLE IF EXISTS test_run_tags;
DROP TABLE IF EXISTS test_plan_tags;
DROP TABLE IF EXISTS test_case_tags;
