package handlers

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"schoolportal/internal/config"
	"schoolportal/internal/middleware"
	"schoolportal/internal/models"
	"schoolportal/web"
)

// Lagos time is fixed at UTC+1 (Nigeria does not observe daylight saving).
var lagos = time.FixedZone("WAT", 3600)

type Flash struct{ Kind, Message string }

type NavItem struct {
	Label  string
	Path   string
	Prefix bool
	Active bool
}

type PageData struct {
	Title      string
	SchoolName string
	Path       string
	User       *models.User
	CSRF       string
	Nav        []NavItem
	Flash      *Flash
	Images     config.Images
	Data       any
}

type Renderer struct {
	pages  map[string]*template.Template
	school string
}

func NewRenderer(school string) (*Renderer, error) {
	funcs := template.FuncMap{
		"date":     func(t time.Time) string { return t.In(lagos).Format("2 Jan 2006") },
		"datetime": func(t time.Time) string { return t.In(lagos).Format("2 Jan 2006, 3:04 pm") },
		"num":      func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) },
		"f1": func(v float64) string {
			return strings.TrimSuffix(strconv.FormatFloat(v, 'f', 1, 64), ".0")
		},
		"plural": func(n int, one, many string) string {
			if n == 1 {
				return "1 " + one
			}
			return strconv.Itoa(n) + " " + many
		},
		"add": func(a, b int) int { return a + b },
		"humanize": func(s string) string {
			s = strings.ReplaceAll(s, "_", " ")
			if s == "" {
				return s
			}
			return strings.ToUpper(s[:1]) + s[1:]
		},
		"pctof": func(part, whole int) int {
			if whole <= 0 {
				return 0
			}
			return part * 100 / whole
		},
		"ratio": func(v, max float64) string {
			if max <= 0 {
				return "0"
			}
			return strconv.FormatFloat(v*100/max, 'f', 1, 64)
		},
		"vsavg": func(total, avg float64) string {
			d := total - avg
			switch {
			case d > 0.049:
				return "You are " + strings.TrimSuffix(strconv.FormatFloat(d, 'f', 1, 64), ".0") + " points above the class average."
			case d < -0.049:
				return "You are " + strings.TrimSuffix(strconv.FormatFloat(-d, 'f', 1, 64), ".0") + " points below the class average."
			}
			return "You are level with the class average."
		},
		"score": func(v *float64) string {
			if v == nil {
				return ""
			}
			return strconv.FormatFloat(*v, 'f', -1, 64)
		},
		"paragraphs": func(s string) []string {
			var out []string
			for _, p := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n\n") {
				if p = strings.TrimSpace(p); p != "" {
					out = append(out, p)
				}
			}
			return out
		},
	}
	r := &Renderer{pages: map[string]*template.Template{}, school: school}
	err := fs.WalkDir(web.Templates, "templates/pages", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".html") {
			return err
		}
		name := strings.TrimSuffix(strings.TrimPrefix(path, "templates/pages/"), ".html")
		t, err := template.New("").Funcs(funcs).ParseFS(web.Templates,
			"templates/layouts/*.html", "templates/partials/*.html", path)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		r.pages[name] = t
		return nil
	})
	return r, err
}

func (a *App) render(w http.ResponseWriter, r *http.Request, status int, name, title string, data any) {
	tpl, ok := a.View.pages[name]
	if !ok {
		a.Log.Error("missing template", "name", name)
		http.Error(w, "Something went wrong.", http.StatusInternalServerError)
		return
	}
	pd := PageData{Title: title, SchoolName: a.View.school, Path: r.URL.Path, Data: data, Flash: a.takeFlash(w, r), Images: a.Cfg.Images}
	if u := middleware.UserFrom(r.Context()); u != nil {
		pd.User = u
		pd.Nav = navFor(u.Role, r.URL.Path)
	}
	if s := middleware.SessionFrom(r.Context()); s != nil {
		pd.CSRF = s.CSRF
	}
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "base", pd); err != nil {
		a.Log.Error("render failed", "name", name, "error", err)
		http.Error(w, "Something went wrong.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

// Flash messages travel in a short-lived cookie and are shown once.
func (a *App) setFlash(w http.ResponseWriter, kind, msg string) {
	http.SetCookie(w, &http.Cookie{Name: "flash", Value: base64.RawURLEncoding.EncodeToString([]byte(kind + "|" + msg)),
		Path: "/", MaxAge: 60, HttpOnly: true, Secure: a.Cfg.SecureCookies, SameSite: http.SameSiteLaxMode})
}

func (a *App) takeFlash(w http.ResponseWriter, r *http.Request) *Flash {
	c, err := r.Cookie("flash")
	if err != nil {
		return nil
	}
	http.SetCookie(w, &http.Cookie{Name: "flash", Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
		Secure: a.Cfg.SecureCookies, SameSite: http.SameSiteLaxMode})
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil || len(raw) > 400 {
		return nil
	}
	kind, msg, ok := strings.Cut(string(raw), "|")
	if !ok || (kind != "ok" && kind != "error") {
		return nil
	}
	return &Flash{Kind: kind, Message: msg}
}

func navFor(role models.Role, path string) []NavItem {
	var items []NavItem
	switch role {
	case models.RolePrincipal:
		items = []NavItem{
			{Label: "Overview", Path: "/principal"},
			{Label: "Analytics", Path: "/principal/analytics"},
			{Label: "Results", Path: "/principal/results", Prefix: true},
			{Label: "Students", Path: "/manage/students", Prefix: true},
			{Label: "Staff", Path: "/manage/staff", Prefix: true},
			{Label: "Administrators", Path: "/principal/admins", Prefix: true},
			{Label: "Academics", Path: "/manage/academics", Prefix: true},
			{Label: "Announcements", Path: "/announcements", Prefix: true},
		}
	case models.RoleAdmin:
		items = []NavItem{
			{Label: "Overview", Path: "/admin"},
			{Label: "Students", Path: "/manage/students", Prefix: true},
			{Label: "Staff", Path: "/manage/staff", Prefix: true},
			{Label: "Academics", Path: "/manage/academics", Prefix: true},
			{Label: "Announcements", Path: "/announcements", Prefix: true},
		}
	case models.RoleTeacher:
		items = []NavItem{
			{Label: "My classes", Path: "/teacher", Prefix: false},
			{Label: "All my students", Path: "/teacher/students"},
			{Label: "Bank details", Path: "/teacher/bank"},
			{Label: "Announcements", Path: "/announcements", Prefix: true},
		}
	case models.RoleSupport:
		items = []NavItem{
			{Label: "My panel", Path: "/support"},
			{Label: "Announcements", Path: "/announcements"},
		}
	case models.RoleStudent:
		items = []NavItem{
			{Label: "My report", Path: "/student"},
			{Label: "Past terms", Path: "/student/results"},
			{Label: "Announcements", Path: "/announcements"},
		}
	}
	for i := range items {
		items[i].Active = path == items[i].Path || (items[i].Prefix && strings.HasPrefix(path, items[i].Path+"/"))
	}
	// A roster page belongs to "My classes".
	if role == models.RoleTeacher && strings.HasPrefix(path, "/teacher/scores/") {
		items[0].Active = true
	}
	return items
}
