-- +goose Up
CREATE TABLE visual_baselines (
    id            BIGSERIAL PRIMARY KEY,
    test_case_id  BIGINT NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
    attachment_id BIGINT NOT NULL REFERENCES attachments(id) ON DELETE CASCADE,
    label         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_visual_baselines_case ON visual_baselines(test_case_id);

CREATE TABLE visual_comparisons (
    id            BIGSERIAL PRIMARY KEY,
    execution_id  BIGINT NOT NULL REFERENCES test_executions(id) ON DELETE CASCADE,
    baseline_id   BIGINT NOT NULL REFERENCES visual_baselines(id) ON DELETE CASCADE,
    attachment_id BIGINT NOT NULL REFERENCES attachments(id) ON DELETE CASCADE, -- actual screenshot
    diff_percent  DOUBLE PRECISION NOT NULL DEFAULT 0,
    verdict       TEXT NOT NULL DEFAULT 'match', -- 'match' | 'mismatch' | 'no-baseline'
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_visual_comparisons_exec ON visual_comparisons(execution_id);

-- +goose Down
DROP TABLE IF EXISTS visual_comparisons;
DROP TABLE IF EXISTS visual_baselines;
