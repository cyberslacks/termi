CREATE TABLE IF NOT EXISTS scheduled_jobs (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT    NOT NULL,
    cron_expr    TEXT    NOT NULL,
    playbook_id  INTEGER NOT NULL REFERENCES playbooks(id) ON DELETE CASCADE,
    group_id     INTEGER REFERENCES groups(id) ON DELETE SET NULL,
    session_ids  TEXT    NOT NULL DEFAULT '[]',
    variables    TEXT    NOT NULL DEFAULT '{}',
    mode         TEXT    NOT NULL DEFAULT 'interactive' CHECK(mode IN ('interactive','autonomous')),
    enabled      INTEGER NOT NULL DEFAULT 1,
    last_run_at  DATETIME,
    next_run_at  DATETIME,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
