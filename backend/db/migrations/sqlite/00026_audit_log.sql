-- +goose Up
CREATE TABLE audit_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_type TEXT NOT NULL,
    entity_id   INTEGER NOT NULL,
    action      TEXT NOT NULL,
    actor_id    INTEGER REFERENCES users(id) ON DELETE SET NULL,
    actor_name  TEXT NOT NULL DEFAULT '',
    old_values  TEXT NOT NULL DEFAULT '{}',
    new_values  TEXT NOT NULL DEFAULT '{}',
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX audit_log_entity_idx ON audit_log (entity_type, entity_id, id DESC);
CREATE INDEX audit_log_actor_idx ON audit_log (actor_id, id DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
