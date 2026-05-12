CREATE TABLE IF NOT EXISTS audit_log (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id     INTEGER NOT NULL,
    session_name   TEXT    NOT NULL DEFAULT '',
    host           TEXT    NOT NULL DEFAULT '',
    actor          TEXT    NOT NULL CHECK(actor IN ('user','agent','scheduler')),
    actor_detail   TEXT    NOT NULL DEFAULT '',
    command        TEXT    NOT NULL,
    exit_code      INTEGER,
    output_snippet TEXT    NOT NULL DEFAULT '',
    playbook_id    INTEGER REFERENCES playbooks(id) ON DELETE SET NULL,
    step_order     INTEGER,
    executed_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at   DATETIME
);

CREATE INDEX IF NOT EXISTS idx_audit_session  ON audit_log(session_id);
CREATE INDEX IF NOT EXISTS idx_audit_executed ON audit_log(executed_at);
CREATE INDEX IF NOT EXISTS idx_audit_actor    ON audit_log(actor);
