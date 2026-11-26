-- +goose Up
CREATE TABLE notifications (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type  TEXT NOT NULL,
    entity_type TEXT NOT NULL DEFAULT '',
    entity_id   BIGINT NOT NULL DEFAULT 0,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL DEFAULT '',
    read_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_notifications_user ON notifications(user_id, read_at, id);

-- +goose Down
DROP INDEX IF EXISTS idx_notifications_user;
DROP TABLE IF EXISTS notifications;
