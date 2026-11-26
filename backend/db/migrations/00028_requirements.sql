-- +goose Up
CREATE TABLE requirements (
    id          BIGSERIAL PRIMARY KEY,
    product_id  BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    key         TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    version     TEXT NOT NULL DEFAULT '1',
    status      TEXT NOT NULL DEFAULT 'draft', -- 'draft' | 'approved' | 'obsolete'
    priority    TEXT NOT NULL DEFAULT 'medium', -- 'low' | 'medium' | 'high' | 'critical'
    category    TEXT NOT NULL DEFAULT '',
    author_id   BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_requirements_product_key ON requirements(product_id, key);
CREATE INDEX idx_requirements_product ON requirements(product_id);

CREATE TABLE requirement_coverage (
    requirement_id BIGINT NOT NULL REFERENCES requirements(id) ON DELETE CASCADE,
    test_case_id   BIGINT NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (requirement_id, test_case_id)
);
CREATE INDEX idx_coverage_test_case ON requirement_coverage(test_case_id);

-- +goose Down
DROP TABLE IF EXISTS requirement_coverage;
DROP INDEX IF EXISTS idx_requirements_product;
DROP INDEX IF EXISTS idx_requirements_product_key;
DROP TABLE IF EXISTS requirements;
