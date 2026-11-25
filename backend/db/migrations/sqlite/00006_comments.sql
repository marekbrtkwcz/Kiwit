-- +goose Up
CREATE TABLE comments (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_type TEXT NOT NULL, -- 'test_case' | 'test_execution'
    entity_id   INTEGER NOT NULL,
    author_id   INTEGER REFERENCES users(id) ON DELETE SET NULL,
    body        TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_comments_entity ON comments(entity_type, entity_id);

-- +goose Down
DROP TABLE IF EXISTS comments;
