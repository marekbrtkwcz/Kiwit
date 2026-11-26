-- +goose Up
CREATE TABLE user_groups (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE user_group_members (
    group_id INTEGER NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    user_id  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX idx_user_group_members_user ON user_group_members(user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_user_group_members_user;
DROP TABLE IF EXISTS user_group_members;
DROP TABLE IF EXISTS user_groups;
