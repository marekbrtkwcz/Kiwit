-- +goose Up
CREATE TABLE categories (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO categories (name) VALUES
    ('Functional'),
    ('Web API'),
    ('Performance'),
    ('UX / Usability'),
    ('Security'),
    ('Smoke'),
    ('Regression');

ALTER TABLE test_cases ADD COLUMN category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL;

-- +goose Down
DROP TABLE IF EXISTS categories;
