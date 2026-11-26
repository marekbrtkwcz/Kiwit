-- +goose Up
CREATE TABLE audit_log (
    id          BIGSERIAL PRIMARY KEY,
    entity_type TEXT NOT NULL,
    entity_id   BIGINT NOT NULL,
    action      TEXT NOT NULL,
    actor_id    BIGINT REFERENCES users(id) ON DELETE SET NULL,
    actor_name  TEXT NOT NULL DEFAULT '',
    old_values  JSONB NOT NULL DEFAULT '{}'::jsonb,
    new_values  JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_entity_idx ON audit_log (entity_type, entity_id, id DESC);
CREATE INDEX audit_log_actor_idx ON audit_log (actor_id, id DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
