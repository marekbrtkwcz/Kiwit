-- +goose Up
CREATE TABLE requirements (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    product_id  INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    key         TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    version     TEXT NOT NULL DEFAULT '1',
    status      TEXT NOT NULL DEFAULT 'draft',
    priority    TEXT NOT NULL DEFAULT 'medium',
    category    TEXT NOT NULL DEFAULT '',
    author_id   INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX idx_requirements_product_key ON requirements(product_id, key);
CREATE INDEX idx_requirements_product ON requirements(product_id);

CREATE TABLE requirement_coverage (
    requirement_id INTEGER NOT NULL REFERENCES requirements(id) ON DELETE CASCADE,
    test_case_id   INTEGER NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (requirement_id, test_case_id)
);
CREATE INDEX idx_coverage_test_case ON requirement_coverage(test_case_id);

-- +goose Down
DROP TABLE IF EXISTS requirement_coverage;
DROP INDEX IF EXISTS idx_requirements_product;
DROP INDEX IF EXISTS idx_requirements_product_key;
DROP TABLE IF EXISTS requirements;
