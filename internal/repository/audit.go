package repository

import (
	"context"
	"time"
)

type AuditEntry struct {
	When   time.Time
	Actor  string
	Action string
	Detail string
}

// Audit writes an entry. userID 0 records an anonymous event (for example a failed sign-in).
func (s *Store) Audit(ctx context.Context, userID int64, action, detail, ip string) error {
	var uid any
	if userID != 0 {
		uid = userID
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO audit_log(user_id, action, detail, ip) VALUES (?, ?, ?, ?)`, uid, action, detail, ip)
	return err
}

func (s *Store) RecentAudit(ctx context.Context, limit int) ([]AuditEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.created_at, COALESCE(u.full_name, 'Anonymous'), a.action, a.detail
		FROM audit_log a LEFT JOIN users u ON u.id = a.user_id ORDER BY a.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var t int64
		if err := rows.Scan(&t, &e.Actor, &e.Action, &e.Detail); err != nil {
			return nil, err
		}
		e.When = ts(t)
		out = append(out, e)
	}
	return out, rows.Err()
}
