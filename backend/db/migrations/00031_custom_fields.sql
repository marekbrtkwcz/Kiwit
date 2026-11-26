-- +goose Up
CREATE TABLE custom_field_defs (
    id          BIGSERIAL PRIMARY KEY,
    entity_type TEXT NOT NULL, -- 'test_case' | 'test_run' | 'test_execution' | 'requirement' | 'milestone'
    name        TEXT NOT NULL,
    field_type  TEXT NOT NULL DEFAULT 'text', -- 'text' | 'number' | 'date' | 'list' | 'multiselect' | 'boolean'
    options     TEXT NOT NULL DEFAULT '[]', -- JSON array for list/multiselect
    required    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (entity_type, name)
);

CREATE TABLE custom_field_values (
    id          BIGSERIAL PRIMARY KEY,
    field_id    BIGINT NOT NULL REFERENCES custom_field_defs(id) ON DELETE CASCADE,
    entity_type TEXT NOT NULL,
    entity_id   BIGINT NOT NULL,
    value       TEXT NOT NULL DEFAULT '',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (field_id, entity_type, entity_id)
);
CREATE INDEX idx_cf_values_entity ON custom_field_values(entity_type, entity_id);

-- +goose Down
DROP TABLE IF EXISTS custom_field_values;
DROP TABLE IF EXISTS custom_field_defs;
