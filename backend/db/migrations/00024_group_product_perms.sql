-- +goose Up
CREATE TABLE group_product_permissions (
    group_id   BIGINT NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('viewer', 'tester', 'maintainer')),
    PRIMARY KEY (group_id, product_id)
);
CREATE INDEX idx_group_product_perms_product ON group_product_permissions(product_id);

-- +goose Down
DROP INDEX IF EXISTS idx_group_product_perms_product;
DROP TABLE IF EXISTS group_product_permissions;
