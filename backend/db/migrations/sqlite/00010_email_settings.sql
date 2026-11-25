-- +goose Up
CREATE TABLE email_settings (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_type      TEXT NOT NULL, -- 'test_case' | 'test_plan'
    entity_id        INTEGER NOT NULL,
    notify_on_update INTEGER NOT NULL DEFAULT 1,
    cc_list          TEXT NOT NULL DEFAULT '',
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (entity_type, entity_id)
);

-- +goose Down
DROP TABLE IF EXISTS email_settings;
