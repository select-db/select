CREATE INDEX idx_permission_role_id ON permission(role_id);

CREATE UNIQUE INDEX idx_permission_unique
    ON permission(role_id, COALESCE(db_instance_id, ''), COALESCE(schema_name, ''), COALESCE(table_name, ''), COALESCE(column_name, ''), action);

CREATE INDEX idx_permission_updated_at ON permission(updated_at);

CREATE INDEX idx_role_updated_at ON role(updated_at);

CREATE INDEX idx_user_to_role_updated_at ON user_to_role(updated_at);

CREATE INDEX idx_user_to_role_user_workspace ON user_to_role(user_id, workspace_id);

CREATE INDEX idx_group_updated_at ON "group"(updated_at);

CREATE INDEX idx_group_workspace_id ON "group"(workspace_id);

CREATE INDEX idx_user_to_group_updated_at ON user_to_group(updated_at);

CREATE INDEX idx_user_to_group_user_workspace ON user_to_group(user_id, workspace_id);

CREATE INDEX idx_user_to_group_group_id ON user_to_group(group_id);

CREATE INDEX idx_group_to_role_updated_at ON group_to_role(updated_at);

CREATE INDEX idx_group_to_role_group_id ON group_to_role(group_id);

CREATE TABLE "group" (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    name         TEXT NOT NULL,
    source       TEXT NOT NULL DEFAULT 'local',
    external_id  TEXT,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at   DATETIME,
    FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE CASCADE
);

CREATE TABLE group_to_role (
    id           TEXT PRIMARY KEY,
    group_id     TEXT NOT NULL,
    role_id      TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at   DATETIME,
    UNIQUE (group_id, role_id),
    FOREIGN KEY (group_id) REFERENCES "group"(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES role(id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE CASCADE
);

CREATE TABLE user_to_group (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL,
    group_id     TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    source       TEXT NOT NULL DEFAULT 'local',
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at   DATETIME,
    UNIQUE (user_id, group_id),
    FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE,
    FOREIGN KEY (group_id) REFERENCES "group"(id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE CASCADE
);

CREATE TABLE goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TIMESTAMP DEFAULT (datetime('now'))
	);

CREATE TABLE history (
    id TEXT PRIMARY KEY,

    statement TEXT NOT NULL,
    affected_rows INTEGER,
    row_count INTEGER,
    duration_ms INTEGER,
    errors TEXT NOT NULL DEFAULT '[]',

    uri TEXT NOT NULL DEFAULT "",
    dsn TEXT NOT NULL DEFAULT "",

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    workspace_id TEXT NOT NULL DEFAULT '',
    db_instance_id TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_history_workspace_created_at ON history(workspace_id, created_at DESC);

CREATE TABLE mutation_commit (
    id TEXT PRIMARY KEY,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    operation TEXT NOT NULL,
    table_name TEXT NOT NULL,
    object_id TEXT NOT NULL,
    payload JSON NOT NULL,

    user_id TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE CASCADE,

    CONSTRAINT unique_sync UNIQUE (operation, table_name, object_id, user_id, workspace_id)
);

CREATE TABLE permission (
    id              TEXT PRIMARY KEY,
    role_id         TEXT NOT NULL,
    workspace_id    TEXT NOT NULL,
    -- NULL across all four db fields = app-level rule (workspace settings, user management, etc.)
    -- '*' in schema/table/column = wildcard (applies to all at that level)
    db_instance_id  TEXT,
    schema_name     TEXT,
    table_name      TEXT,
    column_name     TEXT,
    action          TEXT NOT NULL,
    effect          TEXT NOT NULL DEFAULT 'deny',
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at      DATETIME,
    FOREIGN KEY (role_id) REFERENCES role(id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE CASCADE
);

CREATE TABLE role (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    name         TEXT NOT NULL,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at   DATETIME,
    FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE CASCADE,
    UNIQUE (workspace_id, name)
);

CREATE TABLE user (
    id TEXT PRIMARY KEY,
    name TEXT
, email TEXT, avatar_url TEXT);

CREATE TABLE user_to_role (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL,
    role_id      TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at   DATETIME,
    UNIQUE (user_id, role_id),
    FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES role(id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE CASCADE
);

CREATE TABLE workspace (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    git_remote_url TEXT,

    last_pulled_at DATETIME
, owner_id TEXT REFERENCES user(id) ON DELETE SET NULL, statement_timeout_ms INTEGER NOT NULL DEFAULT 30000, max_result_size_mb INTEGER NOT NULL DEFAULT 100);

CREATE TABLE workspace_to_user (
    id TEXT PRIMARY KEY,
    current BOOLEAN DEFAULT FALSE,

    workspace_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    FOREIGN KEY(workspace_id) REFERENCES workspace(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES user(id) ON DELETE CASCADE,

    UNIQUE(user_id, workspace_id)
);
