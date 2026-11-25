-- +goose Up
CREATE TABLE priorities (
    id         BIGSERIAL PRIMARY KEY,
    value      TEXT NOT NULL UNIQUE,
    is_active  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO priorities (value) VALUES ('low'), ('medium'), ('high'), ('critical');

CREATE TABLE execution_statuses (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    color      TEXT NOT NULL DEFAULT '#6b7280',
    weight     INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO execution_statuses (name, color, weight) VALUES
    ('idle', '#9ca3af', 0),
    ('running', '#2563eb', 10),
    ('passed', '#16a34a', 20),
    ('failed', '#dc2626', -20),
    ('blocked', '#d97706', -10),
    ('skipped', '#6b7280', 0);

-- +goose Down
DROP TABLE IF EXISTS execution_statuses;
DROP TABLE IF EXISTS priorities;
