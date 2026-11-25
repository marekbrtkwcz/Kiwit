-- +goose Up
CREATE TABLE attachments (
    id           BIGSERIAL PRIMARY KEY,
    entity_type  TEXT NOT NULL,
    entity_id    BIGINT NOT NULL,
    filename     TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
    size_bytes   BIGINT NOT NULL,
    storage_path TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_attachments_entity ON attachments(entity_type, entity_id);

CREATE TABLE test_case_revisions (
    id             BIGSERIAL PRIMARY KEY,
    test_case_id   BIGINT NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
    version        INT NOT NULL,
    summary        TEXT NOT NULL,
    preconditions  TEXT NOT NULL DEFAULT '',
    format         TEXT NOT NULL DEFAULT 'steps',
    gherkin_body   TEXT NOT NULL DEFAULT '',
    priority       TEXT NOT NULL DEFAULT 'medium',
    status         TEXT NOT NULL DEFAULT 'confirmed',
    steps_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (test_case_id, version)
);

CREATE TABLE issue_trackers (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    provider   TEXT NOT NULL,
    base_url   TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE bugs (
    id                BIGSERIAL PRIMARY KEY,
    test_execution_id BIGINT NOT NULL REFERENCES test_executions(id) ON DELETE CASCADE,
    tracker_id        BIGINT REFERENCES issue_trackers(id) ON DELETE SET NULL,
    external_key      TEXT NOT NULL,
    url               TEXT NOT NULL,
    title             TEXT NOT NULL DEFAULT '',
    status            TEXT NOT NULL DEFAULT 'open',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_bugs_execution ON bugs(test_execution_id);

CREATE TABLE categories (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO categories (name) VALUES
    ('Functional'),
    ('Web API'),
    ('Performance'),
    ('UX / Usability'),
    ('Security'),
    ('Smoke'),
    ('Regression');

ALTER TABLE test_cases ADD COLUMN category_id BIGINT REFERENCES categories(id) ON DELETE SET NULL;

CREATE TABLE comments (
    id          BIGSERIAL PRIMARY KEY,
    entity_type TEXT NOT NULL,
    entity_id   BIGINT NOT NULL,
    author_id   BIGINT REFERENCES users(id) ON DELETE SET NULL,
    body        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_comments_entity ON comments(entity_type, entity_id);

CREATE TABLE product_members (
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'tester',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (product_id, user_id)
);

-- +goose Down
DROP TABLE IF EXISTS product_members;
DROP TABLE IF EXISTS comments;
ALTER TABLE test_cases DROP COLUMN IF EXISTS category_id;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS bugs;
DROP TABLE IF EXISTS issue_trackers;
DROP TABLE IF EXISTS test_case_revisions;
DROP TABLE IF EXISTS attachments;
