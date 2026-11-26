-- +goose Up
CREATE TABLE notifications (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type  TEXT NOT NULL,
    entity_type TEXT NOT NULL DEFAULT '',
    entity_id   INTEGER NOT NULL DEFAULT 0,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL DEFAULT '',
    read_at     DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_notifications_user ON notifications(user_id, read_at, id);

-- +goose Down
DROP INDEX IF EXISTS idx_notifications_user;
DROP TABLE IF EXISTS notifications;
