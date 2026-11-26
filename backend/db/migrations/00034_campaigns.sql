-- +goose Up
CREATE TABLE campaigns (
    id          BIGSERIAL PRIMARY KEY,
    product_id  BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active', -- 'active' | 'completed' | 'archived'
    start_date  TEXT,
    end_date    TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_campaigns_product ON campaigns(product_id);

CREATE TABLE campaign_iterations (
    id          BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'planned', -- 'planned' | 'in_progress' | 'done'
    start_date  TEXT,
    end_date    TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_iterations_campaign ON campaign_iterations(campaign_id);

ALTER TABLE test_plans ADD COLUMN campaign_id BIGINT REFERENCES campaigns(id) ON DELETE SET NULL;
ALTER TABLE test_plans ADD COLUMN iteration_id BIGINT REFERENCES campaign_iterations(id) ON DELETE SET NULL;
CREATE INDEX idx_test_plans_campaign ON test_plans(campaign_id);
CREATE INDEX idx_test_plans_iteration ON test_plans(iteration_id);

-- +goose Down
DROP INDEX IF EXISTS idx_test_plans_iteration;
DROP INDEX IF EXISTS idx_test_plans_campaign;
ALTER TABLE test_plans DROP COLUMN IF EXISTS iteration_id;
ALTER TABLE test_plans DROP COLUMN IF EXISTS campaign_id;
DROP TABLE IF EXISTS campaign_iterations;
DROP TABLE IF EXISTS campaigns;
