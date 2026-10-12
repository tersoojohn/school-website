package repository

import (
	"context"
	"database/sql"

	"schoolportal/internal/models"
)

func (s *Store) ListClasses(ctx context.Context) ([]models.Class, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name FROM classes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Class
	for rows.Next() {
		var c models.Class
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateClass(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO classes(name) VALUES (?)`, name)
	return mapErr(err)
}

func (s *Store) ClassExists(ctx context.Context, id int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM classes WHERE id = ?`, id).Scan(&n)
	return n > 0, err
}

func (s *Store) ListSubjects(ctx context.Context) ([]models.Subject, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, code FROM subjects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Subject
	for rows.Next() {
		var sb models.Subject
		if err := rows.Scan(&sb.ID, &sb.Name, &sb.Code); err != nil {
			return nil, err
		}
		out = append(out, sb)
	}
	return out, rows.Err()
}

func (s *Store) CreateSubject(ctx context.Context, name, code string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO subjects(name, code) VALUES (?, ?)`, name, code)
	return mapErr(err)
}

func (s *Store) ListTerms(ctx context.Context) ([]models.Term, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, is_current FROM terms ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Term
	for rows.Next() {
		var t models.Term
		var cur int
		if err := rows.Scan(&t.ID, &t.Name, &cur); err != nil {
			return nil, err
		}
		t.IsCurrent = cur == 1
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) CreateTerm(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO terms(name) VALUES (?)`, name)
	return mapErr(err)
}

func (s *Store) SetCurrentTerm(ctx context.Context, id int64) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE terms SET is_current = 0`); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE terms SET is_current = 1 WHERE id = ?`, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return sql.ErrNoRows
		}
		return nil
	})
}

// CurrentTerm returns ErrNotFound when no term is marked current.
func (s *Store) CurrentTerm(ctx context.Context) (*models.Term, error) {
	var t models.Term
	err := s.db.QueryRowContext(ctx, `SELECT id, name FROM terms WHERE is_current = 1`).Scan(&t.ID, &t.Name)
	if err != nil {
		return nil, mapErr(err)
	}
	t.IsCurrent = true
	return &t, nil
}

const classSubjectSelect = `SELECT cs.id, cs.class_id, c.name, cs.subject_id, sb.name, COALESCE(cs.teacher_id, 0), COALESCE(u.full_name, '')
	FROM class_subjects cs
	JOIN classes c ON c.id = cs.class_id
	JOIN subjects sb ON sb.id = cs.subject_id
	LEFT JOIN users u ON u.id = cs.teacher_id `

func scanClassSubject(r rowScanner) (*models.ClassSubject, error) {
	var cs models.ClassSubject
	err := r.Scan(&cs.ID, &cs.ClassID, &cs.ClassName, &cs.SubjectID, &cs.SubjectName, &cs.TeacherID, &cs.TeacherName)
	if err != nil {
		return nil, mapErr(err)
	}
	return &cs, nil
}

// ListClassSubjects returns assignments. teacherID 0 returns all of them.
func (s *Store) ListClassSubjects(ctx context.Context, teacherID int64) ([]models.ClassSubject, error) {
	q := classSubjectSelect
	var args []any
	if teacherID != 0 {
		q += `WHERE cs.teacher_id = ? `
		args = append(args, teacherID)
	}
	q += `ORDER BY c.name, sb.name`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ClassSubject
	for rows.Next() {
		cs, err := scanClassSubject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *cs)
	}
	return out, rows.Err()
}

func (s *Store) GetClassSubject(ctx context.Context, id int64) (*models.ClassSubject, error) {
	return scanClassSubject(s.db.QueryRowContext(ctx, classSubjectSelect+`WHERE cs.id = ?`, id))
}

// AssignClassSubject creates or updates a class/subject pairing. teacherID 0 clears the teacher.
func (s *Store) AssignClassSubject(ctx context.Context, classID, subjectID, teacherID int64) error {
	var t any
	if teacherID != 0 {
		t = teacherID
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO class_subjects(class_id, subject_id, teacher_id) VALUES (?, ?, ?)
		ON CONFLICT(class_id, subject_id) DO UPDATE SET teacher_id = excluded.teacher_id`, classID, subjectID, t)
	return mapErr(err)
}

func (s *Store) IsActiveTeacher(ctx context.Context, id int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id = ? AND role = 'teacher' AND active = 1`, id).Scan(&n)
	return n > 0, err
}
