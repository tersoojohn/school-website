package repository

import (
	"context"
	"database/sql"
	"time"

	"schoolportal/internal/models"
	"schoolportal/internal/services"
)

type ScoreRow struct {
	StudentID int64
	FullName  string
	Username  string
	CA        *float64
	Exam      *float64
	Status    string // draft | submitted
}

type ScoreInput struct {
	StudentID int64
	CA, Exam  *float64
}

// ListScoreSheet returns every active student in the class with any scores already recorded.
func (s *Store) ListScoreSheet(ctx context.Context, termID, classSubjectID int64) ([]ScoreRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT st.user_id, u.full_name, u.username, r.ca_score, r.exam_score, COALESCE(r.status, 'draft')
		FROM class_subjects cs
		JOIN students st ON st.class_id = cs.class_id
		JOIN users u ON u.id = st.user_id AND u.active = 1
		LEFT JOIN results r ON r.student_id = st.user_id AND r.class_subject_id = cs.id AND r.term_id = ?
		WHERE cs.id = ?
		ORDER BY u.full_name`, termID, classSubjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScoreRow
	for rows.Next() {
		var r ScoreRow
		var ca, ex sql.NullFloat64
		if err := rows.Scan(&r.StudentID, &r.FullName, &r.Username, &ca, &ex, &r.Status); err != nil {
			return nil, err
		}
		if ca.Valid {
			r.CA = &ca.Float64
		}
		if ex.Valid {
			r.Exam = &ex.Float64
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SaveScores records draft scores. Rows that are already submitted are never overwritten.
func (s *Store) SaveScores(ctx context.Context, termID, classSubjectID, teacherID int64, in []ScoreInput) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		now := time.Now().Unix()
		for _, e := range in {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO results(term_id, class_subject_id, student_id, ca_score, exam_score, status, recorded_by, updated_at)
				VALUES (?, ?, ?, ?, ?, 'draft', ?, ?)
				ON CONFLICT(term_id, class_subject_id, student_id) DO UPDATE SET
					ca_score = excluded.ca_score, exam_score = excluded.exam_score,
					recorded_by = excluded.recorded_by, updated_at = excluded.updated_at
				WHERE results.status = 'draft'`,
				termID, classSubjectID, e.StudentID, e.CA, e.Exam, teacherID, now); err != nil {
				return err
			}
		}
		return nil
	})
}

// SubmitSheet locks the sheet and makes it visible to students. Every active student
// must have both scores, otherwise ErrIncomplete is returned and nothing changes.
func (s *Store) SubmitSheet(ctx context.Context, termID, classSubjectID int64) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		var total, complete int
		err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*), COUNT(CASE WHEN r.ca_score IS NOT NULL AND r.exam_score IS NOT NULL THEN 1 END)
			FROM class_subjects cs
			JOIN students st ON st.class_id = cs.class_id
			JOIN users u ON u.id = st.user_id AND u.active = 1
			LEFT JOIN results r ON r.student_id = st.user_id AND r.class_subject_id = cs.id AND r.term_id = ?
			WHERE cs.id = ?`, termID, classSubjectID).Scan(&total, &complete)
		if err != nil {
			return err
		}
		if total == 0 || complete != total {
			return ErrIncomplete
		}
		_, err = tx.ExecContext(ctx, `UPDATE results SET status = 'submitted', updated_at = ?
			WHERE term_id = ? AND class_subject_id = ? AND status = 'draft'`, time.Now().Unix(), termID, classSubjectID)
		return err
	})
}

// ReopenSheet returns a submitted sheet to draft so the teacher can correct it.
func (s *Store) ReopenSheet(ctx context.Context, termID, classSubjectID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE results SET status = 'draft', updated_at = ?
		WHERE term_id = ? AND class_subject_id = ?`, time.Now().Unix(), termID, classSubjectID)
	return err
}

type SheetProgress struct {
	ClassSubjectID int64
	ClassName      string
	SubjectName    string
	TeacherName    string
	Students       int
	Scored         int
	Submitted      int
}

func (p SheetProgress) Status() string {
	switch {
	case p.Students > 0 && p.Submitted >= p.Students:
		return "Submitted"
	case p.Scored > 0 || p.Submitted > 0:
		return "In progress"
	}
	return "Not started"
}

func (p SheetProgress) IsSubmitted() bool { return p.Status() == "Submitted" }

// SheetOverview reports score entry progress for a term. teacherID 0 returns every sheet.
func (s *Store) SheetOverview(ctx context.Context, termID, teacherID int64) ([]SheetProgress, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cs.id, c.name, sb.name, COALESCE(u.full_name, ''),
			(SELECT COUNT(*) FROM students st JOIN users su ON su.id = st.user_id AND su.active = 1 WHERE st.class_id = cs.class_id),
			(SELECT COUNT(*) FROM results r JOIN users ru ON ru.id = r.student_id AND ru.active = 1
				WHERE r.class_subject_id = cs.id AND r.term_id = ? AND r.ca_score IS NOT NULL AND r.exam_score IS NOT NULL),
			(SELECT COUNT(*) FROM results r JOIN users ru ON ru.id = r.student_id AND ru.active = 1
				WHERE r.class_subject_id = cs.id AND r.term_id = ? AND r.status = 'submitted')
		FROM class_subjects cs
		JOIN classes c ON c.id = cs.class_id
		JOIN subjects sb ON sb.id = cs.subject_id
		LEFT JOIN users u ON u.id = cs.teacher_id
		WHERE (? = 0 OR cs.teacher_id = ?)
		ORDER BY c.name, sb.name`, termID, termID, teacherID, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SheetProgress
	for rows.Next() {
		var p SheetProgress
		if err := rows.Scan(&p.ClassSubjectID, &p.ClassName, &p.SubjectName, &p.TeacherName, &p.Students, &p.Scored, &p.Submitted); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// StudentResults returns only submitted scores for one student and term.
func (s *Store) StudentResults(ctx context.Context, studentID, termID int64) ([]services.RawResult, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT sb.name, r.ca_score, r.exam_score
		FROM results r
		JOIN class_subjects cs ON cs.id = r.class_subject_id
		JOIN subjects sb ON sb.id = cs.subject_id
		WHERE r.student_id = ? AND r.term_id = ? AND r.status = 'submitted'
			AND r.ca_score IS NOT NULL AND r.exam_score IS NOT NULL
		ORDER BY sb.name`, studentID, termID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []services.RawResult
	for rows.Next() {
		var r services.RawResult
		if err := rows.Scan(&r.Subject, &r.CA, &r.Exam); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// StudentTerms lists the terms in which the student has at least one submitted result.
func (s *Store) StudentTerms(ctx context.Context, studentID int64) ([]models.Term, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.name, t.is_current FROM terms t
		WHERE EXISTS (SELECT 1 FROM results r WHERE r.term_id = t.id AND r.student_id = ? AND r.status = 'submitted')
		ORDER BY t.id DESC`, studentID)
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
