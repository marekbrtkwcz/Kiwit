-- +goose Up
CREATE TABLE classifications (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);
ALTER TABLE products ADD COLUMN classification_id INTEGER REFERENCES classifications(id) ON DELETE SET NULL;
CREATE INDEX idx_products_classification ON products(classification_id);

-- +goose Down
DROP INDEX IF EXISTS idx_products_classification;
ALTER TABLE products DROP COLUMN classification_id;
DROP TABLE IF EXISTS classifications;
