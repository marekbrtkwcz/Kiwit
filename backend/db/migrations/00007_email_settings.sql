-- +goose Up
CREATE TABLE email_settings (
    id               BIGSERIAL PRIMARY KEY,
    entity_type      TEXT NOT NULL,
    entity_id        BIGINT NOT NULL,
    notify_on_update BOOLEAN NOT NULL DEFAULT TRUE,
    cc_list          TEXT NOT NULL DEFAULT '',
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (entity_type, entity_id)
);

-- +goose Down
DROP TABLE IF EXISTS email_settings;
