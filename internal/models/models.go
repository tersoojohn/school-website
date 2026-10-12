// Package models defines the plain data types shared across layers.
package models

import "time"

type Role string

const (
	RolePrincipal Role = "principal"
	RoleAdmin     Role = "admin"
	RoleTeacher   Role = "teacher"
	RoleSupport   Role = "support"
	RoleStudent   Role = "student"
)

func (r Role) Valid() bool {
	switch r {
	case RolePrincipal, RoleAdmin, RoleTeacher, RoleSupport, RoleStudent:
		return true
	}
	return false
}

func (r Role) Label() string {
	switch r {
	case RolePrincipal:
		return "Principal"
	case RoleAdmin:
		return "Administrator"
	case RoleTeacher:
		return "Teacher"
	case RoleSupport:
		return "Support staff"
	case RoleStudent:
		return "Student"
	}
	return string(r)
}

// Home is the landing page for each role.
func (r Role) Home() string {
	switch r {
	case RolePrincipal:
		return "/principal"
	case RoleAdmin:
		return "/admin"
	case RoleTeacher:
		return "/teacher"
	case RoleSupport:
		return "/support"
	case RoleStudent:
		return "/student"
	}
	return "/login"
}

// CanManage reports whether a user with this role may view, reset or deactivate
// accounts of the target role. The principal manages everyone else; administrators
// manage students and staff but never other administrators or the principal.
func (r Role) CanManage(target Role) bool {
	switch r {
	case RolePrincipal:
		return target != RolePrincipal
	case RoleAdmin:
		return target == RoleStudent || target == RoleTeacher || target == RoleSupport
	}
	return false
}

func (r Role) CanPostAnnouncements() bool {
	return r == RolePrincipal || r == RoleAdmin || r == RoleTeacher
}

type StaffKind string

const (
	StaffEducating    StaffKind = "educating"
	StaffNonEducating StaffKind = "non_educating"
)

type User struct {
	ID                 int64
	Username           string
	FullName           string
	Role               Role
	MustChangePassword bool
	Active             bool
	Phone              string
	CreatedAt          time.Time
	LastLoginAt        *time.Time
}

type Session struct {
	TokenHash string
	UserID    int64
	CSRF      string
	LastSeen  time.Time
	ExpiresAt time.Time
}

type Student struct {
	User
	ClassID       int64
	ClassName     string
	GuardianName  string
	GuardianPhone string
	DateOfBirth   string
	Gender        string
}

type Staff struct {
	User
	Kind     StaffKind
	JobTitle string
}

type Term struct {
	ID        int64
	Name      string
	IsCurrent bool
}

type Class struct {
	ID   int64
	Name string
}

type Subject struct {
	ID   int64
	Name string
	Code string
}

type ClassSubject struct {
	ID          int64
	ClassID     int64
	ClassName   string
	SubjectID   int64
	SubjectName string
	TeacherID   int64 // 0 when unassigned
	TeacherName string
}

func (c ClassSubject) Label() string { return c.SubjectName + ", " + c.ClassName }

type Announcement struct {
	ID          int64
	Title       string
	Body        string
	Scope       string
	CreatedAt   time.Time
	AuthorID    int64
	AuthorName  string
	AuthorRole  Role
	ClassName   string
	SubjectName string
	CanDelete   bool
}

func (a Announcement) Audience() string {
	switch a.Scope {
	case "school":
		return "Everyone"
	case "staff":
		return "All staff"
	}
	return a.SubjectName + ", " + a.ClassName
}

type BankDetails struct {
	BankName      string
	AccountName   string
	AccountNumber string // decrypted; only populated for authorised viewers
	Last4         string
	UpdatedAt     time.Time
}
