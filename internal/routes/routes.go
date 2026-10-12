// Package routes is the single place that maps URLs to handlers and decides who may reach them.
package routes

import (
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"schoolportal/internal/config"
	"schoolportal/internal/handlers"
	"schoolportal/internal/middleware"
	"schoolportal/internal/models"
	"schoolportal/internal/repository"
	"schoolportal/web"
)

const (
	principal = models.RolePrincipal
	admin     = models.RoleAdmin
	teacher   = models.RoleTeacher
	support   = models.RoleSupport
	student   = models.RoleStudent
)

func New(app *handlers.App, store *repository.Store, cfg *config.Config, log *slog.Logger) (http.Handler, error) {
	mux := http.NewServeMux()

	// protect = signed in, then role check, then CSRF. With no roles listed, any signed-in user may pass.
	protect := func(h http.HandlerFunc, roles ...models.Role) http.Handler {
		mws := []middleware.Middleware{middleware.RequireLogin}
		if len(roles) > 0 {
			mws = append(mws, middleware.RequireRole(app.Forbidden, roles...))
		}
		mws = append(mws, middleware.CSRF(app.Forbidden, app.Origin))
		return middleware.Chain(h, mws...)
	}

	// ---- static files ----
	static, err := fs.Sub(web.Static, "static")
	if err != nil {
		return nil, err
	}
	files := http.StripPrefix("/static/", http.FileServerFS(static))
	mux.Handle("GET /static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		files.ServeHTTP(w, r)
	}))

	// ---- public ----
	loginLimit := middleware.NewLimiter(10, time.Minute).Middleware(cfg.TrustProxy)
	activateLimit := middleware.NewLimiter(15, time.Minute).Middleware(cfg.TrustProxy)
	mux.HandleFunc("GET /login", app.LoginForm)
	mux.HandleFunc("GET /activate/{token}", app.ActivateForm)
	mux.Handle("POST /activate/{token}", activateLimit(http.HandlerFunc(app.ActivateSubmit)))
	mux.Handle("POST /login", loginLimit(http.HandlerFunc(app.Login)))

	// ---- any signed-in user ----
	passwordLimit := middleware.NewLimiter(10, time.Minute).Middleware(cfg.TrustProxy)
	mux.Handle("GET /{$}", protect(app.Home))
	mux.Handle("POST /logout", protect(app.Logout))
	mux.Handle("GET /account/password", protect(app.PasswordForm))
	mux.Handle("POST /account/password", passwordLimit(protect(app.PasswordSubmit)))
	mux.Handle("GET /announcements", protect(app.Announcements))

	// ---- announcements: only principal, administrators and teachers may write ----
	poster := []models.Role{principal, admin, teacher}
	mux.Handle("GET /announcements/new", protect(app.AnnouncementForm, poster...))
	mux.Handle("POST /announcements", protect(app.AnnouncementCreate, poster...))
	mux.Handle("POST /announcements/{id}/delete", protect(app.AnnouncementDelete, poster...))

	// ---- principal ----
	mux.Handle("GET /principal", protect(app.PrincipalDashboard, principal))
	mux.Handle("GET /principal/analytics", protect(app.Analytics, principal))
	mux.Handle("GET /principal/results", protect(app.ResultsOverview, principal))
	mux.Handle("POST /principal/results/{id}/reopen", protect(app.ReopenSheet, principal))
	mux.Handle("GET /principal/admins", protect(app.Admins, principal))
	mux.Handle("POST /principal/admins", protect(app.AdminCreate, principal))

	// ---- administrative staff ----
	mux.Handle("GET /admin", protect(app.AdminDashboard, admin))

	// ---- shared by principal and administrators: registration and school structure ----
	m := []models.Role{principal, admin}
	mux.Handle("GET /manage/students", protect(app.Students, m...))
	mux.Handle("GET /manage/students/new", protect(app.StudentForm, m...))
	mux.Handle("POST /manage/students", protect(app.StudentCreate, m...))
	mux.Handle("GET /manage/staff", protect(app.StaffList, m...))
	mux.Handle("GET /manage/staff/new", protect(app.StaffForm, m...))
	mux.Handle("POST /manage/staff", protect(app.StaffCreate, m...))
	mux.Handle("GET /manage/people/{id}", protect(app.Person, m...))
	mux.Handle("POST /manage/people/{id}/setup-link", protect(app.ResetPassword, m...))
	mux.Handle("POST /manage/people/{id}/active", protect(app.SetActive, m...))
	mux.Handle("GET /manage/academics", protect(app.Academics, m...))
	mux.Handle("POST /manage/academics/classes", protect(app.ClassCreate, m...))
	mux.Handle("POST /manage/academics/subjects", protect(app.SubjectCreate, m...))
	mux.Handle("POST /manage/academics/terms", protect(app.TermCreate, m...))
	mux.Handle("POST /manage/academics/terms/{id}/current", protect(app.TermSetCurrent, m...))
	mux.Handle("POST /manage/academics/assignments", protect(app.Assign, m...))

	// ---- educating staff ----
	mux.Handle("GET /teacher", protect(app.TeacherDashboard, teacher))
	mux.Handle("GET /teacher/students", protect(app.TeacherStudents, teacher))
	mux.Handle("GET /teacher/students/export.csv", protect(app.TeacherStudentsCSV, teacher))
	mux.Handle("GET /teacher/scores/{id}", protect(app.ScoresForm, teacher))
	mux.Handle("GET /teacher/scores/{id}/export.csv", protect(app.ScoresCSV, teacher))
	mux.Handle("POST /teacher/scores/{id}", protect(app.ScoresSave, teacher))
	mux.Handle("GET /teacher/bank", protect(app.BankForm, teacher))
	mux.Handle("POST /teacher/bank", protect(app.BankSave, teacher))

	// ---- non-educating staff ----
	mux.Handle("GET /support", protect(app.SupportDashboard, support))

	// ---- students ----
	mux.Handle("GET /student", protect(app.StudentDashboard, student))
	mux.Handle("GET /student/results", protect(app.StudentResults, student))

	mux.HandleFunc("/", app.NotFound)

	return middleware.Chain(mux,
		middleware.Recover(log),
		middleware.Logger(log, cfg.TrustProxy),
		middleware.SecureHeaders(cfg.SecureCookies, cfg.ImageHosts()),
		middleware.LimitBody,
		middleware.Authenticate(store, cfg),
	), nil
}
