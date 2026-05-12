package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type DBPlaybook struct {
	ID          int64
	Name        string
	Description string
	Tags        []string
	FilePath    string     // non-empty → Ansible playbook on disk
	TrustedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PlaybookRepo struct{ db *DB }

func NewPlaybookRepo(db *DB) *PlaybookRepo { return &PlaybookRepo{db} }

func (r *PlaybookRepo) Create(ctx context.Context, p *DBPlaybook) error {
	tags, _ := json.Marshal(p.Tags)
	row := r.db.QueryRowContext(ctx,
		`INSERT INTO playbooks(name,description,tags,file_path,variables) VALUES(?,?,?,?,?)
		 RETURNING id,created_at,updated_at`,
		p.Name, p.Description, string(tags), p.FilePath, "[]")
	return row.Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *PlaybookRepo) Update(ctx context.Context, p *DBPlaybook) error {
	tags, _ := json.Marshal(p.Tags)
	_, err := r.db.ExecContext(ctx,
		`UPDATE playbooks SET name=?,description=?,tags=?,file_path=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		p.Name, p.Description, string(tags), p.FilePath, p.ID)
	return err
}

func (r *PlaybookRepo) Get(ctx context.Context, id int64) (*DBPlaybook, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id,name,description,tags,file_path,trusted_at,created_at,updated_at FROM playbooks WHERE id=?`, id)
	return scanPlaybook(row)
}

func (r *PlaybookRepo) List(ctx context.Context) ([]DBPlaybook, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,name,description,tags,file_path,trusted_at,created_at,updated_at FROM playbooks ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DBPlaybook
	for rows.Next() {
		p, err := scanPlaybook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (r *PlaybookRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM playbooks WHERE id=?`, id)
	return err
}

func (r *PlaybookRepo) Trust(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE playbooks SET trusted_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	return err
}

func (r *PlaybookRepo) Untrust(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE playbooks SET trusted_at=NULL WHERE id=?`, id)
	return err
}

func scanPlaybook(s scanner) (*DBPlaybook, error) {
	var p DBPlaybook
	var tagsJSON string
	var trustedAt sql.NullTime
	err := s.Scan(&p.ID, &p.Name, &p.Description, &tagsJSON, &p.FilePath,
		&trustedAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if trustedAt.Valid {
		p.TrustedAt = &trustedAt.Time
	}
	_ = json.Unmarshal([]byte(tagsJSON), &p.Tags)
	return &p, nil
}
