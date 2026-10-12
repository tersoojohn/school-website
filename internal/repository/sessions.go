package repository

import (
	"context"
	"time"

	"schoolportal/internal/models"
)

func (s *Store) CreateSession(ctx context.Context, sess models.Session, ip, userAgent string) error {
	if len(userAgent) > 255 {
		userAgent = userAgent[:255]
	}
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions(token_hash, user_id, csrf_token, created_at, last_seen_at, expires_at, ip, user_agent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.TokenHash, sess.UserID, sess.CSRF, now, sess.LastSeen.Unix(), sess.ExpiresAt.Unix(), ip, userAgent)
	return err
}

func (s *Store) GetSession(ctx context.Context, tokenHash string) (*models.Session, *models.User, error) {
	var csrf string
	var seen, expires int64
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT s.csrf_token, s.last_seen_at, s.expires_at, `+userCols+`
		FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token_hash = ?`, tokenHash), &csrf, &seen, &expires)
	if err != nil {
		return nil, nil, err
	}
	return &models.Session{TokenHash: tokenHash, UserID: u.ID, CSRF: csrf, LastSeen: ts(seen), ExpiresAt: ts(expires)}, u, nil
}

func (s *Store) TouchSession(ctx context.Context, tokenHash string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ? WHERE token_hash = ?`, now.Unix(), tokenHash)
	return err
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) DeleteUserSessions(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// PurgeSessions removes expired and idle sessions.
func (s *Store) PurgeSessions(ctx context.Context, now time.Time, idle time.Duration) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ? OR last_seen_at < ?`,
		now.Unix(), now.Add(-idle).Unix())
	return err
}
