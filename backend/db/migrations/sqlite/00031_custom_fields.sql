-- +goose Up
CREATE TABLE custom_field_defs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_type TEXT NOT NULL,
    name        TEXT NOT NULL,
    field_type  TEXT NOT NULL DEFAULT 'text',
    options     TEXT NOT NULL DEFAULT '[]',
    required    INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (entity_type, name)
);

CREATE TABLE custom_field_values (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    field_id    INTEGER NOT NULL REFERENCES custom_field_defs(id) ON DELETE CASCADE,
    entity_type TEXT NOT NULL,
    entity_id   INTEGER NOT NULL,
    value       TEXT NOT NULL DEFAULT '',
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (field_id, entity_type, entity_id)
);
CREATE INDEX idx_cf_values_entity ON custom_field_values(entity_type, entity_id);

-- +goose Down
DROP TABLE IF EXISTS custom_field_values;
DROP TABLE IF EXISTS custom_field_defs;
