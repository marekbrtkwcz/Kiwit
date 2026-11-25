-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT NOT NULL UNIQUE,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'tester',
    is_active     INTEGER NOT NULL DEFAULT 1,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE api_tokens (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    expires_at   DATETIME,
    last_used_at DATETIME,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE products (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE versions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    product_id  INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    value       TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (product_id, value)
);

CREATE TABLE builds (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    version_id  INTEGER NOT NULL REFERENCES versions(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    is_active   INTEGER NOT NULL DEFAULT 1,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (version_id, name)
);

CREATE TABLE components (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    product_id  INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (product_id, name)
);

CREATE TABLE tags (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE test_cases (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    product_id    INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    summary       TEXT NOT NULL,
    preconditions TEXT NOT NULL DEFAULT '',
    format        TEXT NOT NULL DEFAULT 'steps',
    gherkin_body  TEXT NOT NULL DEFAULT '',
    priority      TEXT NOT NULL DEFAULT 'medium',
    status        TEXT NOT NULL DEFAULT 'confirmed',
    author_id     INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE test_case_steps (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    test_case_id    INTEGER NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
    position        INTEGER NOT NULL,
    action          TEXT NOT NULL,
    expected_result TEXT NOT NULL DEFAULT '',
    UNIQUE (test_case_id, position)
);

CREATE TABLE test_plans (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    product_id  INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    version_id  INTEGER NOT NULL REFERENCES versions(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_active   INTEGER NOT NULL DEFAULT 1,
    author_id   INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE test_plan_cases (
    test_plan_id INTEGER NOT NULL REFERENCES test_plans(id) ON DELETE CASCADE,
    test_case_id INTEGER NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
    position     INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (test_plan_id, test_case_id)
);

CREATE TABLE test_runs (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    test_plan_id INTEGER NOT NULL REFERENCES test_plans(id) ON DELETE CASCADE,
    build_id     INTEGER NOT NULL REFERENCES builds(id) ON DELETE RESTRICT,
    summary      TEXT NOT NULL,
    notes        TEXT NOT NULL DEFAULT '',
    manager_id   INTEGER REFERENCES users(id) ON DELETE SET NULL,
    started_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    stopped_at   DATETIME
);

CREATE TABLE test_executions (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    test_run_id    INTEGER NOT NULL REFERENCES test_runs(id) ON DELETE CASCADE,
    test_case_id   INTEGER NOT NULL REFERENCES test_cases(id) ON DELETE RESTRICT,
    status         TEXT NOT NULL DEFAULT 'idle',
    assignee_id    INTEGER REFERENCES users(id) ON DELETE SET NULL,
    tested_by_id   INTEGER REFERENCES users(id) ON DELETE SET NULL,
    steps_snapshot TEXT NOT NULL DEFAULT '[]',
    comment        TEXT NOT NULL DEFAULT '',
    executed_at    DATETIME,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS test_executions;
DROP TABLE IF EXISTS test_runs;
DROP TABLE IF EXISTS test_plan_cases;
DROP TABLE IF EXISTS test_plans;
DROP TABLE IF EXISTS test_case_steps;
DROP TABLE IF EXISTS test_cases;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS components;
DROP TABLE IF EXISTS builds;
DROP TABLE IF EXISTS versions;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS api_tokens;
DROP TABLE IF EXISTS users;
