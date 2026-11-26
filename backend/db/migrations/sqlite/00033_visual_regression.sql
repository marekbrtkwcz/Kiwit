-- +goose Up
CREATE TABLE visual_baselines (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    test_case_id  INTEGER NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
    attachment_id INTEGER NOT NULL REFERENCES attachments(id) ON DELETE CASCADE,
    label         TEXT NOT NULL DEFAULT '',
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_visual_baselines_case ON visual_baselines(test_case_id);

CREATE TABLE visual_comparisons (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    execution_id  INTEGER NOT NULL REFERENCES test_executions(id) ON DELETE CASCADE,
    baseline_id   INTEGER NOT NULL REFERENCES visual_baselines(id) ON DELETE CASCADE,
    attachment_id INTEGER NOT NULL REFERENCES attachments(id) ON DELETE CASCADE,
    diff_percent  REAL NOT NULL DEFAULT 0,
    verdict       TEXT NOT NULL DEFAULT 'match',
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_visual_comparisons_exec ON visual_comparisons(execution_id);

-- +goose Down
DROP TABLE IF EXISTS visual_comparisons;
DROP TABLE IF EXISTS visual_baselines;
