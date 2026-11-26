-- +goose Up
CREATE TABLE user_notification_settings (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    in_app     BOOLEAN NOT NULL DEFAULT TRUE,
    email      BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, event_type)
);

-- +goose Down
DROP TABLE IF EXISTS user_notification_settings;
