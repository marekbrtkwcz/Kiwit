-- +goose Up
CREATE TABLE test_execution_tags (
    test_execution_id INTEGER NOT NULL REFERENCES test_executions(id) ON DELETE CASCADE,
    tag_id            INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (test_execution_id, tag_id)
);

-- +goose Down
DROP TABLE IF EXISTS test_execution_tags;
