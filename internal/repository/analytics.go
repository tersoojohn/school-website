package repository

import (
	"context"
	"time"

	"schoolportal/internal/services"
)

type Count struct {
	Label string
	Value int
}

type SubjectStat struct {
	Subject  string
	Results  int
	Mean     float64
	PassRate float64
}

type ClassStat struct {
	Class string
	Mean  float64
}

type TermStat struct {
	Term string
	Mean float64
}

const submittedScores = `r.status = 'submitted' AND r.ca_score IS NOT NULL AND r.exam_score IS NOT NULL`

// Enrollment returns active students per class, including empty classes.
func (s *Store) Enrollment(ctx context.Context) ([]Count, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.name, COUNT(u.id) FROM classes c
		LEFT JOIN students st ON st.class_id = c.id
		LEFT JOIN users u ON u.id = st.user_id AND u.active = 1
		GROUP BY c.id ORDER BY c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Count
	for rows.Next() {
		var c Count
		if err := rows.Scan(&c.Label, &c.Value); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// RegistrationsByMonth counts new accounts (students and staff) over the last n months, oldest first.
func (s *Store) RegistrationsByMonth(ctx context.Context, n int) ([]Count, error) {
	now := time.Now()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -(n - 1), 0)
	rows, err := s.db.QueryContext(ctx, `SELECT strftime('%Y-%m', created_at, 'unixepoch'), COUNT(*) FROM users
		WHERE role IN ('student','teacher','support') AND created_at >= ? GROUP BY 1`, first.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byMonth := map[string]int{}
	for rows.Next() {
		var m string
		var c int
		if err := rows.Scan(&m, &c); err != nil {
			return nil, err
		}
		byMonth[m] = c
	}
	out := make([]Count, 0, n)
	for i := 0; i < n; i++ {
		t := first.AddDate(0, i, 0)
		out = append(out, Count{Label: t.Format("Jan"), Value: byMonth[t.Format("2006-01")]})
	}
	return out, rows.Err()
}

// Totals returns the total score of every submitted result. Filters are optional (0 = no filter).
func (s *Store) Totals(ctx context.Context, termID, classSubjectID, teacherID int64) ([]float64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT r.ca_score + r.exam_score FROM results r
		JOIN class_subjects cs ON cs.id = r.class_subject_id
		WHERE `+submittedScores+` AND r.term_id = ?
			AND (? = 0 OR r.class_subject_id = ?) AND (? = 0 OR cs.teacher_id = ?)`,
		termID, classSubjectID, classSubjectID, teacherID, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []float64
	for rows.Next() {
		var v float64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// SubjectStats gives each subject's average and pass rate across all classes for a term.
func (s *Store) SubjectStats(ctx context.Context, termID int64) ([]SubjectStat, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT sb.name, COUNT(*), AVG(r.ca_score + r.exam_score),
			100.0 * AVG(CASE WHEN r.ca_score + r.exam_score >= ? THEN 1.0 ELSE 0.0 END)
		FROM results r JOIN class_subjects cs ON cs.id = r.class_subject_id JOIN subjects sb ON sb.id = cs.subject_id
		WHERE `+submittedScores+` AND r.term_id = ?
		GROUP BY sb.id ORDER BY AVG(r.ca_score + r.exam_score) DESC`, services.PassMark, termID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SubjectStat
	for rows.Next() {
		var st SubjectStat
		if err := rows.Scan(&st.Subject, &st.Results, &st.Mean, &st.PassRate); err != nil {
			return nil, err
		}
		st.Mean, st.PassRate = services.Round1(st.Mean), services.Round1(st.PassRate)
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Store) ClassStats(ctx context.Context, termID int64) ([]ClassStat, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.name, AVG(r.ca_score + r.exam_score)
		FROM results r JOIN class_subjects cs ON cs.id = r.class_subject_id JOIN classes c ON c.id = cs.class_id
		WHERE `+submittedScores+` AND r.term_id = ? GROUP BY c.id ORDER BY c.name`, termID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ClassStat
	for rows.Next() {
		var c ClassStat
		if err := rows.Scan(&c.Class, &c.Mean); err != nil {
			return nil, err
		}
		c.Mean = services.Round1(c.Mean)
		out = append(out, c)
	}
	return out, rows.Err()
}

// TermTrend returns the school-wide average for each term that has submitted results, oldest first.
func (s *Store) TermTrend(ctx context.Context) ([]TermStat, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.name, AVG(r.ca_score + r.exam_score)
		FROM results r JOIN terms t ON t.id = r.term_id WHERE `+submittedScores+` GROUP BY t.id ORDER BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TermStat
	for rows.Next() {
		var t TermStat
		if err := rows.Scan(&t.Term, &t.Mean); err != nil {
			return nil, err
		}
		t.Mean = services.Round1(t.Mean)
		out = append(out, t)
	}
	return out, rows.Err()
}

// ---- teacher ----

type RosterEntry struct {
	StudentID int64
	FullName  string
	Username  string
	Class     string
	Subjects  string
}

// TeacherRoster lists every active student the teacher is assigned, once each,
// with the subjects they take from this teacher.
func (s *Store) TeacherRoster(ctx context.Context, teacherID int64) ([]RosterEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT u.id, u.full_name, u.username, c.name, GROUP_CONCAT(sb.name, ', ')
		FROM class_subjects cs
		JOIN subjects sb ON sb.id = cs.subject_id
		JOIN classes c ON c.id = cs.class_id
		JOIN students st ON st.class_id = cs.class_id
		JOIN users u ON u.id = st.user_id AND u.active = 1
		WHERE cs.teacher_id = ?
		GROUP BY u.id ORDER BY c.name, u.full_name`, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RosterEntry
	for rows.Next() {
		var r RosterEntry
		if err := rows.Scan(&r.StudentID, &r.FullName, &r.Username, &r.Class, &r.Subjects); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---- student ----

type ClassAverage struct {
	Mean float64
	N    int
}

// ClassAverages returns each subject's class average for a term. Subjects with fewer than
// minN submitted results are left out so one student's score is never exposed as "the average".
func (s *Store) ClassAverages(ctx context.Context, classID, termID int64, minN int) (map[string]ClassAverage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT sb.name, AVG(r.ca_score + r.exam_score), COUNT(*)
		FROM results r JOIN class_subjects cs ON cs.id = r.class_subject_id JOIN subjects sb ON sb.id = cs.subject_id
		WHERE `+submittedScores+` AND cs.class_id = ? AND r.term_id = ? GROUP BY sb.id`, classID, termID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]ClassAverage{}
	for rows.Next() {
		var name string
		var ca ClassAverage
		if err := rows.Scan(&name, &ca.Mean, &ca.N); err != nil {
			return nil, err
		}
		if ca.N >= minN {
			ca.Mean = services.Round1(ca.Mean)
			out[name] = ca
		}
	}
	return out, rows.Err()
}

// StudentTermAverages is one student's average per term, oldest first.
func (s *Store) StudentTermAverages(ctx context.Context, studentID int64) ([]TermStat, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.name, AVG(r.ca_score + r.exam_score)
		FROM results r JOIN terms t ON t.id = r.term_id
		WHERE `+submittedScores+` AND r.student_id = ? GROUP BY t.id ORDER BY t.id`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TermStat
	for rows.Next() {
		var t TermStat
		if err := rows.Scan(&t.Term, &t.Mean); err != nil {
			return nil, err
		}
		t.Mean = services.Round1(t.Mean)
		out = append(out, t)
	}
	return out, rows.Err()
}
