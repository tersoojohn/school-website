package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"schoolportal/internal/models"
)

const userCols = `u.id, u.username, u.full_name, u.role, u.must_change_password, u.active, u.phone, u.created_at, u.last_login_at`

// scanUser reads userCols; extra destinations may be placed before them.
func scanUser(r rowScanner, pre ...any) (*models.User, error) {
	var (
		u            models.User
		role         string
		must, active int
		created      int64
		last         sql.NullInt64
	)
	dest := append(pre, &u.ID, &u.Username, &u.FullName, &role, &must, &active, &u.Phone, &created, &last)
	if err := r.Scan(dest...); err != nil {
		return nil, mapErr(err)
	}
	u.Role = models.Role(role)
	u.MustChangePassword = must == 1
	u.Active = active == 1
	u.CreatedAt = ts(created)
	if last.Valid {
		t := ts(last.Int64)
		u.LastLoginAt = &t
	}
	return &u, nil
}

func (s *Store) GetUser(ctx context.Context, id int64) (*models.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users u WHERE u.id = ?`, id))
}

type LoginRecord struct {
	User        *models.User
	Hash        string
	LockedUntil time.Time
}

func (s *Store) GetUserForLogin(ctx context.Context, username string) (*LoginRecord, error) {
	var (
		hash   string
		locked int64
	)
	u, err := scanUser(s.db.QueryRowContext(ctx,
		`SELECT u.password_hash, u.locked_until, `+userCols+` FROM users u WHERE u.username = ?`, username), &hash, &locked)
	if err != nil {
		return nil, err
	}
	return &LoginRecord{User: u, Hash: hash, LockedUntil: ts(locked)}, nil
}

// RecordFailedLogin counts a failure and locks the account once maxAttempts is reached.
func (s *Store) RecordFailedLogin(ctx context.Context, id int64, maxAttempts int, lockFor time.Duration) error {
	until := time.Now().Add(lockFor).Unix()
	_, err := s.db.ExecContext(ctx, `UPDATE users SET
		locked_until    = CASE WHEN failed_attempts + 1 >= ? THEN ? ELSE locked_until END,
		failed_attempts = CASE WHEN failed_attempts + 1 >= ? THEN 0 ELSE failed_attempts + 1 END
		WHERE id = ?`, maxAttempts, until, maxAttempts, id)
	return err
}

func (s *Store) RecordLogin(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET failed_attempts = 0, locked_until = 0, last_login_at = ? WHERE id = ?`, time.Now().Unix(), id)
	return err
}

func (s *Store) UnlockUser(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET failed_attempts = 0, locked_until = 0 WHERE id = ?`, id)
	return err
}

func (s *Store) GetPasswordHash(ctx context.Context, id int64) (string, error) {
	var h string
	err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, id).Scan(&h)
	return h, mapErr(err)
}

func (s *Store) UpdatePassword(ctx context.Context, id int64, hash string, mustChange bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, must_change_password = ? WHERE id = ?`, hash, b2i(mustChange), id)
	return err
}

func (s *Store) SetActive(ctx context.Context, id int64, active bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET active = ? WHERE id = ?`, b2i(active), id)
	return err
}

func (s *Store) HasPrincipal(ctx context.Context) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'principal'`).Scan(&n)
	return n > 0, err
}

func (s *Store) CountActiveByRole(ctx context.Context) (map[models.Role]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT role, COUNT(*) FROM users WHERE active = 1 GROUP BY role`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[models.Role]int{}
	for rows.Next() {
		var r string
		var n int
		if err := rows.Scan(&r, &n); err != nil {
			return nil, err
		}
		out[models.Role(r)] = n
	}
	return out, rows.Err()
}

// ---- creation ----

type NewUser struct {
	FullName     string
	Phone        string
	PasswordHash string
	CreatedBy    int64 // 0 for none
}

type NewStudent struct {
	ClassID       int64
	GuardianName  string
	GuardianPhone string
	DateOfBirth   string
	Gender        string
}

func nextUsername(ctx context.Context, tx *sql.Tx, prefix string) (string, error) {
	year := time.Now().Year()
	key := fmt.Sprintf("%s%d", prefix, year)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO counters(name, value) VALUES (?, 1) ON CONFLICT(name) DO UPDATE SET value = value + 1`, key); err != nil {
		return "", err
	}
	var v int
	if err := tx.QueryRowContext(ctx, `SELECT value FROM counters WHERE name = ?`, key).Scan(&v); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%d%04d", prefix, year, v), nil
}

func insertUser(ctx context.Context, tx *sql.Tx, username string, role models.Role, nu NewUser) (int64, error) {
	var createdBy any
	if nu.CreatedBy != 0 {
		createdBy = nu.CreatedBy
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO users(username, full_name, role, password_hash, must_change_password, phone, created_by)
		VALUES (?, ?, ?, ?, 1, ?, ?)`, username, nu.FullName, string(role), nu.PasswordHash, nu.Phone, createdBy)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CreatePrincipal inserts the single principal account with a chosen username.
func (s *Store) CreatePrincipal(ctx context.Context, username string, nu NewUser) (int64, error) {
	var id int64
	err := s.tx(ctx, func(tx *sql.Tx) (err error) {
		id, err = insertUser(ctx, tx, username, models.RolePrincipal, nu)
		return err
	})
	return id, err
}

// Created identifies a newly registered account.
type Created struct {
	ID       int64
	Username string
}

func (s *Store) CreateAdmin(ctx context.Context, nu NewUser) (Created, error) {
	var out Created
	err := s.tx(ctx, func(tx *sql.Tx) (err error) {
		if out.Username, err = nextUsername(ctx, tx, "adm"); err != nil {
			return err
		}
		out.ID, err = insertUser(ctx, tx, out.Username, models.RoleAdmin, nu)
		return err
	})
	return out, err
}

func (s *Store) CreateStudent(ctx context.Context, nu NewUser, st NewStudent) (Created, error) {
	var out Created
	err := s.tx(ctx, func(tx *sql.Tx) (err error) {
		if out.Username, err = nextUsername(ctx, tx, "stu"); err != nil {
			return err
		}
		if out.ID, err = insertUser(ctx, tx, out.Username, models.RoleStudent, nu); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO students(user_id, class_id, guardian_name, guardian_phone, date_of_birth, gender)
			VALUES (?, ?, ?, ?, ?, ?)`, out.ID, st.ClassID, st.GuardianName, st.GuardianPhone, st.DateOfBirth, st.Gender)
		return err
	})
	return out, err
}

func (s *Store) CreateStaff(ctx context.Context, nu NewUser, kind models.StaffKind, jobTitle string) (Created, error) {
	role, prefix := models.RoleTeacher, "tch"
	if kind == models.StaffNonEducating {
		role, prefix = models.RoleSupport, "sup"
	}
	var out Created
	err := s.tx(ctx, func(tx *sql.Tx) (err error) {
		if out.Username, err = nextUsername(ctx, tx, prefix); err != nil {
			return err
		}
		if out.ID, err = insertUser(ctx, tx, out.Username, role, nu); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO staff(user_id, kind, job_title) VALUES (?, ?, ?)`, out.ID, string(kind), jobTitle)
		return err
	})
	return out, err
}

// ---- listing ----

func (s *Store) ListAdmins(ctx context.Context) ([]models.User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userCols+` FROM users u WHERE u.role = 'admin' ORDER BY u.full_name`)
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

const studentSelect = `SELECT ` + userCols + `, st.class_id, c.name, st.guardian_name, st.guardian_phone, st.date_of_birth, st.gender
	FROM students st JOIN users u ON u.id = st.user_id JOIN classes c ON c.id = st.class_id `

func scanStudent(r rowScanner) (*models.Student, error) {
	var st models.Student
	var (
		role         string
		must, active int
		created      int64
		last         sql.NullInt64
	)
	err := r.Scan(&st.ID, &st.Username, &st.FullName, &role, &must, &active, &st.Phone, &created, &last,
		&st.ClassID, &st.ClassName, &st.GuardianName, &st.GuardianPhone, &st.DateOfBirth, &st.Gender)
	if err != nil {
		return nil, mapErr(err)
	}
	st.Role = models.Role(role)
	st.MustChangePassword = must == 1
	st.Active = active == 1
	st.CreatedAt = ts(created)
	if last.Valid {
		t := ts(last.Int64)
		st.LastLoginAt = &t
	}
	return &st, nil
}

// ListStudents returns students, optionally limited to one class (classID 0 = all).
func (s *Store) ListStudents(ctx context.Context, classID int64) ([]models.Student, error) {
	q := studentSelect
	var args []any
	if classID != 0 {
		q += `WHERE st.class_id = ? `
		args = append(args, classID)
	}
	q += `ORDER BY c.name, u.full_name`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Student
	for rows.Next() {
		st, err := scanStudent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *st)
	}
	return out, rows.Err()
}

func (s *Store) GetStudent(ctx context.Context, id int64) (*models.Student, error) {
	return scanStudent(s.db.QueryRowContext(ctx, studentSelect+`WHERE st.user_id = ?`, id))
}

const staffSelect = `SELECT ` + userCols + `, sf.kind, sf.job_title FROM staff sf JOIN users u ON u.id = sf.user_id `

func scanStaff(r rowScanner) (*models.Staff, error) {
	var sf models.Staff
	var (
		role, kind   string
		must, active int
		created      int64
		last         sql.NullInt64
	)
	err := r.Scan(&sf.ID, &sf.Username, &sf.FullName, &role, &must, &active, &sf.Phone, &created, &last, &kind, &sf.JobTitle)
	if err != nil {
		return nil, mapErr(err)
	}
	sf.Role = models.Role(role)
	sf.MustChangePassword = must == 1
	sf.Active = active == 1
	sf.CreatedAt = ts(created)
	if last.Valid {
		t := ts(last.Int64)
		sf.LastLoginAt = &t
	}
	sf.Kind = models.StaffKind(kind)
	return &sf, nil
}

// ListStaff returns staff of one kind, or all staff when kind is empty.
func (s *Store) ListStaff(ctx context.Context, kind models.StaffKind) ([]models.Staff, error) {
	q := staffSelect
	var args []any
	if kind != "" {
		q += `WHERE sf.kind = ? `
		args = append(args, string(kind))
	}
	q += `ORDER BY sf.kind, u.full_name`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Staff
	for rows.Next() {
		sf, err := scanStaff(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sf)
	}
	return out, rows.Err()
}

func (s *Store) GetStaff(ctx context.Context, id int64) (*models.Staff, error) {
	return scanStaff(s.db.QueryRowContext(ctx, staffSelect+`WHERE sf.user_id = ?`, id))
}
