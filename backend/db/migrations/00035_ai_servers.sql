-- +goose Up
CREATE TABLE ai_servers (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT NOT NULL UNIQUE,
    provider      TEXT NOT NULL DEFAULT 'custom', -- 'openai' | 'anthropic' | 'mistral' | 'azure' | 'vertex' | 'custom'
    base_url      TEXT NOT NULL,
    model         TEXT NOT NULL,
    api_key       TEXT NOT NULL DEFAULT '',
    temperature   DOUBLE PRECISION NOT NULL DEFAULT 0.3,
    max_tokens    INT NOT NULL DEFAULT 2000,
    timeout_sec   INT NOT NULL DEFAULT 60,
    retries       INT NOT NULL DEFAULT 1,
    is_default    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS ai_servers;
