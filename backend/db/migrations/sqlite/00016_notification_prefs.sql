-- +goose Up
CREATE TABLE user_notification_settings (
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    in_app     BOOLEAN NOT NULL DEFAULT 1,
    email      BOOLEAN NOT NULL DEFAULT 1,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, event_type)
);

-- +goose Down
DROP TABLE IF EXISTS user_notification_settings;
