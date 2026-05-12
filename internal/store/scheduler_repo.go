package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type ApprovalMode string

const (
	ApprovalInteractive ApprovalMode = "interactive"
	ApprovalAutonomous  ApprovalMode = "autonomous"
)

type ScheduledJob struct {
	ID         int64
	Name       string
	CronExpr   string
	PlaybookID int64
	GroupID    *int64
	SessionIDs []int64
	Variables  map[string]string
	Mode       ApprovalMode
	Enabled    bool
	LastRunAt  *time.Time
	NextRunAt  *time.Time
	CreatedAt  time.Time
}

type SchedulerRepo struct{ db *DB }

func NewSchedulerRepo(db *DB) *SchedulerRepo { return &SchedulerRepo{db} }

func (r *SchedulerRepo) Create(ctx context.Context, j *ScheduledJob) error {
	sessionIDs, _ := json.Marshal(j.SessionIDs)
	vars, _ := json.Marshal(j.Variables)
	row := r.db.QueryRowContext(ctx,
		`INSERT INTO scheduled_jobs(name,cron_expr,playbook_id,group_id,session_ids,variables,mode,enabled)
		 VALUES(?,?,?,?,?,?,?,?) RETURNING id,created_at`,
		j.Name, j.CronExpr, j.PlaybookID, j.GroupID,
		string(sessionIDs), string(vars), j.Mode, boolInt(j.Enabled))
	return row.Scan(&j.ID, &j.CreatedAt)
}

func (r *SchedulerRepo) Update(ctx context.Context, j *ScheduledJob) error {
	sessionIDs, _ := json.Marshal(j.SessionIDs)
	vars, _ := json.Marshal(j.Variables)
	_, err := r.db.ExecContext(ctx,
		`UPDATE scheduled_jobs SET name=?,cron_expr=?,playbook_id=?,group_id=?,
		 session_ids=?,variables=?,mode=?,enabled=? WHERE id=?`,
		j.Name, j.CronExpr, j.PlaybookID, j.GroupID,
		string(sessionIDs), string(vars), j.Mode, boolInt(j.Enabled), j.ID)
	return err
}

func (r *SchedulerRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM scheduled_jobs WHERE id=?`, id)
	return err
}

func (r *SchedulerRepo) Get(ctx context.Context, id int64) (*ScheduledJob, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id,name,cron_expr,playbook_id,group_id,session_ids,variables,mode,enabled,last_run_at,next_run_at,created_at
		 FROM scheduled_jobs WHERE id=?`, id)
	return scanJob(row)
}

func (r *SchedulerRepo) List(ctx context.Context) ([]ScheduledJob, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,name,cron_expr,playbook_id,group_id,session_ids,variables,mode,enabled,last_run_at,next_run_at,created_at
		 FROM scheduled_jobs ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScheduledJob
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

func (r *SchedulerRepo) TouchLastRun(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE scheduled_jobs SET last_run_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	return err
}

func (r *SchedulerRepo) SetNextRun(ctx context.Context, id int64, t time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE scheduled_jobs SET next_run_at=? WHERE id=?`, t, id)
	return err
}

func scanJob(s scanner) (*ScheduledJob, error) {
	var j ScheduledJob
	var groupID sql.NullInt64
	var lastRun, nextRun sql.NullTime
	var enabled int
	var sessionIDsJSON, varsJSON string
	err := s.Scan(&j.ID, &j.Name, &j.CronExpr, &j.PlaybookID, &groupID,
		&sessionIDsJSON, &varsJSON, &j.Mode, &enabled, &lastRun, &nextRun, &j.CreatedAt)
	if err != nil {
		return nil, err
	}
	j.Enabled = enabled != 0
	if groupID.Valid {
		j.GroupID = &groupID.Int64
	}
	if lastRun.Valid {
		j.LastRunAt = &lastRun.Time
	}
	if nextRun.Valid {
		j.NextRunAt = &nextRun.Time
	}
	_ = json.Unmarshal([]byte(sessionIDsJSON), &j.SessionIDs)
	_ = json.Unmarshal([]byte(varsJSON), &j.Variables)
	return &j, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
