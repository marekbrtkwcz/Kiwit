-- +goose Up
CREATE TABLE product_members (
    product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'tester', -- 'viewer' | 'tester' | 'maintainer'
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (product_id, user_id)
);

-- +goose Down
DROP TABLE IF EXISTS product_members;
