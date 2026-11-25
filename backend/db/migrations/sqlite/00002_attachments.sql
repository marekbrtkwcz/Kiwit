-- +goose Up
CREATE TABLE attachments (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_type  TEXT NOT NULL, -- 'test_case' | 'test_execution'
    entity_id    INTEGER NOT NULL,
    filename     TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
    size_bytes   INTEGER NOT NULL,
    storage_path TEXT NOT NULL,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_attachments_entity ON attachments(entity_type, entity_id);

-- +goose Down
DROP TABLE IF EXISTS attachments;
