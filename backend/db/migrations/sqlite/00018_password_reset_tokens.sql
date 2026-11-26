-- +goose Up
CREATE TABLE password_reset_tokens (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    used_at    DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_password_reset_tokens_user ON password_reset_tokens(user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_password_reset_tokens_user;
DROP TABLE IF EXISTS password_reset_tokens;
