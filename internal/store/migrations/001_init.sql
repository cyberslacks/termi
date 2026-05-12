CREATE TABLE IF NOT EXISTS groups (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL UNIQUE,
    description TEXT    NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT    NOT NULL,
    host            TEXT    NOT NULL,
    port            INTEGER NOT NULL DEFAULT 22,
    user            TEXT    NOT NULL,
    auth_method     TEXT    NOT NULL CHECK(auth_method IN ('password','key_file','agent','key_ring')),
    credential_id   TEXT    NOT NULL DEFAULT '',
    jump_host_id    INTEGER REFERENCES sessions(id) ON DELETE SET NULL,
    group_id        INTEGER REFERENCES groups(id) ON DELETE SET NULL,
    notes           TEXT    NOT NULL DEFAULT '',
    tags            TEXT    NOT NULL DEFAULT '[]',
    last_connected  DATETIME,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sessions_group ON sessions(group_id);
CREATE INDEX IF NOT EXISTS idx_sessions_host  ON sessions(host);
