package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type AuthMethod string

const (
	AuthPassword AuthMethod = "password"
	AuthKeyFile  AuthMethod = "key_file"
	AuthAgent    AuthMethod = "agent"
	AuthKeyRing  AuthMethod = "key_ring"
)

type Group struct {
	ID          int64
	Name        string
	Description string
	CreatedAt   time.Time
}

type Session struct {
	ID            int64
	Name          string
	Host          string
	Port          int
	User          string
	AuthMethod    AuthMethod
	CredentialID  string
	JumpHostID    *int64
	GroupID       *int64
	Notes         string
	Tags          []string
	LastConnected *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type SessionRepo struct{ db *DB }

func NewSessionRepo(db *DB) *SessionRepo { return &SessionRepo{db} }

func (r *SessionRepo) CreateGroup(ctx context.Context, g *Group) error {
	row := r.db.QueryRowContext(ctx,
		`INSERT INTO groups(name, description) VALUES(?,?) RETURNING id, created_at`,
		g.Name, g.Description)
	return row.Scan(&g.ID, &g.CreatedAt)
}

func (r *SessionRepo) ListGroups(ctx context.Context) ([]Group, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, description, created_at FROM groups ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *SessionRepo) Create(ctx context.Context, s *Session) error {
	tags, _ := json.Marshal(s.Tags)
	row := r.db.QueryRowContext(ctx,
		`INSERT INTO sessions(name,host,port,user,auth_method,credential_id,jump_host_id,group_id,notes,tags)
		 VALUES(?,?,?,?,?,?,?,?,?,?) RETURNING id, created_at, updated_at`,
		s.Name, s.Host, s.Port, s.User, s.AuthMethod, s.CredentialID,
		s.JumpHostID, s.GroupID, s.Notes, string(tags))
	return row.Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

func (r *SessionRepo) Update(ctx context.Context, s *Session) error {
	tags, _ := json.Marshal(s.Tags)
	_, err := r.db.ExecContext(ctx,
		`UPDATE sessions SET name=?,host=?,port=?,user=?,auth_method=?,credential_id=?,
		 jump_host_id=?,group_id=?,notes=?,tags=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		s.Name, s.Host, s.Port, s.User, s.AuthMethod, s.CredentialID,
		s.JumpHostID, s.GroupID, s.Notes, string(tags), s.ID)
	return err
}

func (r *SessionRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE id=?`, id)
	return err
}

func (r *SessionRepo) Get(ctx context.Context, id int64) (*Session, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id,name,host,port,user,auth_method,credential_id,jump_host_id,group_id,
		        notes,tags,last_connected,created_at,updated_at FROM sessions WHERE id=?`, id)
	return scanSession(row)
}

func (r *SessionRepo) List(ctx context.Context) ([]Session, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,name,host,port,user,auth_method,credential_id,jump_host_id,group_id,
		        notes,tags,last_connected,created_at,updated_at FROM sessions ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func (r *SessionRepo) ListByGroup(ctx context.Context, groupID int64) ([]Session, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,name,host,port,user,auth_method,credential_id,jump_host_id,group_id,
		        notes,tags,last_connected,created_at,updated_at FROM sessions WHERE group_id=? ORDER BY name`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func (r *SessionRepo) TouchConnected(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sessions SET last_connected=CURRENT_TIMESTAMP WHERE id=?`, id)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSession(s scanner) (*Session, error) {
	var sess Session
	var tagsJSON string
	var jumpHostID, groupID sql.NullInt64
	var lastConn sql.NullTime

	err := s.Scan(
		&sess.ID, &sess.Name, &sess.Host, &sess.Port, &sess.User,
		&sess.AuthMethod, &sess.CredentialID,
		&jumpHostID, &groupID,
		&sess.Notes, &tagsJSON,
		&lastConn, &sess.CreatedAt, &sess.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if jumpHostID.Valid {
		sess.JumpHostID = &jumpHostID.Int64
	}
	if groupID.Valid {
		sess.GroupID = &groupID.Int64
	}
	if lastConn.Valid {
		sess.LastConnected = &lastConn.Time
	}
	_ = json.Unmarshal([]byte(tagsJSON), &sess.Tags)
	return &sess, nil
}
