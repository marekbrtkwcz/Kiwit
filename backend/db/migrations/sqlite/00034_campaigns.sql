-- +goose Up
CREATE TABLE campaigns (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    product_id  INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active',
    start_date  TEXT,
    end_date    TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_campaigns_product ON campaigns(product_id);

CREATE TABLE campaign_iterations (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    campaign_id INTEGER NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'planned',
    start_date  TEXT,
    end_date    TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_iterations_campaign ON campaign_iterations(campaign_id);

ALTER TABLE test_plans ADD COLUMN campaign_id INTEGER REFERENCES campaigns(id) ON DELETE SET NULL;
ALTER TABLE test_plans ADD COLUMN iteration_id INTEGER REFERENCES campaign_iterations(id) ON DELETE SET NULL;
CREATE INDEX idx_test_plans_campaign ON test_plans(campaign_id);
CREATE INDEX idx_test_plans_iteration ON test_plans(iteration_id);

-- +goose Down
DROP INDEX IF EXISTS idx_test_plans_iteration;
DROP INDEX IF EXISTS idx_test_plans_campaign;
ALTER TABLE test_plans DROP COLUMN iteration_id;
ALTER TABLE test_plans DROP COLUMN campaign_id;
DROP TABLE IF EXISTS campaign_iterations;
DROP TABLE IF EXISTS campaigns;
