package repository

import (
	"context"
	"time"

	"schoolportal/internal/models"
)

type NewAnnouncement struct {
	AuthorID       int64
	Title, Body    string
	Scope          string
	ClassSubjectID int64 // only for scope "subject"
}

func (s *Store) CreateAnnouncement(ctx context.Context, a NewAnnouncement) error {
	var cs any
	if a.Scope == "subject" {
		cs = a.ClassSubjectID
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO announcements(author_id, title, body, scope, class_subject_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, a.AuthorID, a.Title, a.Body, a.Scope, cs, time.Now().Unix())
	return mapErr(err)
}

// ListAnnouncements returns what the given user is allowed to see:
//   - principal and administrators: everything
//   - teachers: school and staff notices, notices for subjects they teach, and their own
//   - support staff: school and staff notices
//   - students: school notices and notices for subjects taken by their class
func (s *Store) ListAnnouncements(ctx context.Context, u *models.User, limit int) ([]models.Announcement, error) {
	q := `SELECT a.id, a.title, a.body, a.scope, a.created_at, a.author_id, au.full_name, au.role,
			COALESCE(c.name, ''), COALESCE(sb.name, '')
		FROM announcements a
		JOIN users au ON au.id = a.author_id
		LEFT JOIN class_subjects cs ON cs.id = a.class_subject_id
		LEFT JOIN classes c ON c.id = cs.class_id
		LEFT JOIN subjects sb ON sb.id = cs.subject_id `
	var args []any
	switch u.Role {
	case models.RolePrincipal, models.RoleAdmin:
	case models.RoleTeacher:
		q += `WHERE a.scope IN ('school', 'staff') OR a.author_id = ? OR cs.teacher_id = ? `
		args = append(args, u.ID, u.ID)
	case models.RoleSupport:
		q += `WHERE a.scope IN ('school', 'staff') `
	case models.RoleStudent:
		q += `WHERE a.scope = 'school' OR (a.scope = 'subject' AND cs.class_id = (SELECT class_id FROM students WHERE user_id = ?)) `
		args = append(args, u.ID)
	default:
		return nil, nil
	}
	q += `ORDER BY a.created_at DESC, a.id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Announcement
	for rows.Next() {
		var a models.Announcement
		var created int64
		var role string
		if err := rows.Scan(&a.ID, &a.Title, &a.Body, &a.Scope, &created, &a.AuthorID, &a.AuthorName, &role, &a.ClassName, &a.SubjectName); err != nil {
			return nil, err
		}
		a.CreatedAt = ts(created)
		a.AuthorRole = models.Role(role)
		a.CanDelete = u.Role == models.RolePrincipal || u.Role == models.RoleAdmin || a.AuthorID == u.ID
		out = append(out, a)
	}
	return out, rows.Err()
}

// DeleteAnnouncement removes a notice. When onlyAuthor is non-zero, only that author's notice can be removed.
func (s *Store) DeleteAnnouncement(ctx context.Context, id, onlyAuthor int64) error {
	q := `DELETE FROM announcements WHERE id = ?`
	args := []any{id}
	if onlyAuthor != 0 {
		q += ` AND author_id = ?`
		args = append(args, onlyAuthor)
	}
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
