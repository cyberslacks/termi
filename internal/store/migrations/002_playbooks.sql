CREATE TABLE IF NOT EXISTS playbooks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL,
    description TEXT    NOT NULL DEFAULT '',
    tags        TEXT    NOT NULL DEFAULT '[]',
    variables   TEXT    NOT NULL DEFAULT '[]',
    trusted_at  DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS playbook_steps (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    playbook_id INTEGER NOT NULL REFERENCES playbooks(id) ON DELETE CASCADE,
    step_order  INTEGER NOT NULL,
    command     TEXT    NOT NULL,
    timeout_sec INTEGER NOT NULL DEFAULT 30,
    on_error    TEXT    NOT NULL DEFAULT 'abort' CHECK(on_error IN ('abort','continue','retry')),
    retry_count INTEGER NOT NULL DEFAULT 0,
    UNIQUE(playbook_id, step_order)
);

CREATE INDEX IF NOT EXISTS idx_steps_playbook ON playbook_steps(playbook_id);
