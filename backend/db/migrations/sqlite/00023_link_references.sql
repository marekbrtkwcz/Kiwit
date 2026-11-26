-- +goose Up
CREATE TABLE link_references (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('test_run', 'test_execution')),
    entity_id   INTEGER NOT NULL,
    name        TEXT NOT NULL,
    url         TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_link_references_lookup ON link_references (entity_type, entity_id);

-- +goose Down
DROP INDEX IF EXISTS idx_link_references_lookup;
DROP TABLE IF EXISTS link_references;
