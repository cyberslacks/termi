package store

import (
	"context"
	"database/sql"
	"time"
)

type ActorType string

const (
	ActorUser      ActorType = "user"
	ActorAgent     ActorType = "agent"
	ActorScheduler ActorType = "scheduler"
)

type AuditEntry struct {
	ID            int64
	SessionID     int64
	SessionName   string
	Host          string
	Actor         ActorType
	ActorDetail   string
	Command       string
	ExitCode      *int
	OutputSnippet string
	PlaybookID    *int64
	StepOrder     *int
	ExecutedAt    time.Time
	CompletedAt   *time.Time
}

type AuditFilter struct {
	SessionID  *int64
	Actor      *ActorType
	Since      *time.Time
	Limit      int
	Offset     int
}

type AuditRepo struct{ db *DB }

func NewAuditRepo(db *DB) *AuditRepo { return &AuditRepo{db} }

func (r *AuditRepo) Insert(ctx context.Context, e *AuditEntry) error {
	row := r.db.QueryRowContext(ctx,
		`INSERT INTO audit_log(session_id,session_name,host,actor,actor_detail,command,
		 output_snippet,playbook_id,step_order,executed_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?) RETURNING id`,
		e.SessionID, e.SessionName, e.Host, e.Actor, e.ActorDetail,
		e.Command, e.OutputSnippet, e.PlaybookID, e.StepOrder, e.ExecutedAt)
	return row.Scan(&e.ID)
}

func (r *AuditRepo) Complete(ctx context.Context, id int64, exitCode int, outputSnippet string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE audit_log SET exit_code=?,output_snippet=?,completed_at=CURRENT_TIMESTAMP WHERE id=?`,
		exitCode, outputSnippet, id)
	return err
}

func (r *AuditRepo) Query(ctx context.Context, f AuditFilter) ([]AuditEntry, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 200
	}

	q := `SELECT id,session_id,session_name,host,actor,actor_detail,command,exit_code,
	             output_snippet,playbook_id,step_order,executed_at,completed_at
	      FROM audit_log WHERE 1=1`
	var args []any

	if f.SessionID != nil {
		q += " AND session_id=?"
		args = append(args, *f.SessionID)
	}
	if f.Actor != nil {
		q += " AND actor=?"
		args = append(args, *f.Actor)
	}
	if f.Since != nil {
		q += " AND executed_at>=?"
		args = append(args, *f.Since)
	}
	q += " ORDER BY executed_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, f.Offset)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var exitCode sql.NullInt64
		var completedAt sql.NullTime
		var playbookID sql.NullInt64
		var stepOrder sql.NullInt64
		err := rows.Scan(&e.ID, &e.SessionID, &e.SessionName, &e.Host,
			&e.Actor, &e.ActorDetail, &e.Command, &exitCode,
			&e.OutputSnippet, &playbookID, &stepOrder,
			&e.ExecutedAt, &completedAt)
		if err != nil {
			return nil, err
		}
		if exitCode.Valid {
			v := int(exitCode.Int64)
			e.ExitCode = &v
		}
		if completedAt.Valid {
			e.CompletedAt = &completedAt.Time
		}
		if playbookID.Valid {
			e.PlaybookID = &playbookID.Int64
		}
		if stepOrder.Valid {
			v := int(stepOrder.Int64)
			e.StepOrder = &v
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
