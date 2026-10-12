package repository

import (
	"context"
	"database/sql"
	"time"

	"schoolportal/internal/models"
)

// CreateActivation stores a new setup link for the user, replacing any earlier one.
func (s *Store) CreateActivation(ctx context.Context, userID int64, tokenHash string, expires time.Time) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM activation_tokens WHERE user_id = ?`, userID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO activation_tokens(token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
			tokenHash, userID, expires.Unix())
		return err
	})
}

// GetActivation returns the account a valid, unexpired link belongs to.
func (s *Store) GetActivation(ctx context.Context, tokenHash string) (*models.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+`
		FROM activation_tokens t JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = ? AND t.expires_at > ? AND u.active = 1`, tokenHash, time.Now().Unix()))
}

// CompleteActivation sets the password and consumes the link in one transaction,
// so a link can never be used twice.
func (s *Store) CompleteActivation(ctx context.Context, tokenHash, passwordHash string) (*models.User, error) {
	var user *models.User
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var uid int64
		err := tx.QueryRowContext(ctx, `SELECT t.user_id FROM activation_tokens t JOIN users u ON u.id = t.user_id
			WHERE t.token_hash = ? AND t.expires_at > ? AND u.active = 1`, tokenHash, time.Now().Unix()).Scan(&uid)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ?, must_change_password = 0,
			failed_attempts = 0, locked_until = 0 WHERE id = ?`, passwordHash, uid); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM activation_tokens WHERE user_id = ?`, uid); err != nil {
			return err
		}
		user, err = scanUser(tx.QueryRowContext(ctx, `SELECT `+userCols+` FROM users u WHERE u.id = ?`, uid))
		return err
	})
	return user, err
}

// ListPendingSetups returns active accounts still waiting for their owner to set a password.
func (s *Store) ListPendingSetups(ctx context.Context, limit int) ([]models.User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userCols+` FROM users u
		WHERE u.must_change_password = 1 AND u.active = 1
		ORDER BY u.created_at DESC, u.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

// MarkPendingSetup puts an existing account back into "awaiting setup": nobody knows its
// password, every session is ended and a new setup link must be issued.
func (s *Store) MarkPendingSetup(ctx context.Context, userID int64, unusableHash string) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ?, must_change_password = 1,
			failed_attempts = 0, locked_until = 0 WHERE id = ?`, unusableHash, userID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
		return err
	})
}
