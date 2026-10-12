package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"schoolportal/internal/middleware"
	"schoolportal/internal/models"
	"schoolportal/internal/repository"
)

type announcementsView struct {
	Items   []models.Announcement
	CanPost bool
}

func (a *App) Announcements(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	items, err := a.Store.ListAnnouncements(r.Context(), u, 100)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "announcements/list", "Announcements", announcementsView{Items: items, CanPost: u.Role.CanPostAnnouncements()})
}

type announceForm struct {
	Subjects     []models.ClassSubject
	CanBroadcast bool // principal and administrators may address everyone or all staff
	Error        string
	Title, Body  string
	Scope        string
	SubjectID    int64
}

// postableSubjects lists the class/subject pairs the user may address: all of them for
// the principal and administrators, only their own for teachers.
func (a *App) postableSubjects(r *http.Request, u *models.User) ([]models.ClassSubject, error) {
	if u.Role == models.RoleTeacher {
		return a.Store.ListClassSubjects(r.Context(), u.ID)
	}
	return a.Store.ListClassSubjects(r.Context(), 0)
}

func (a *App) AnnouncementForm(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	subjects, err := a.postableSubjects(r, u)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	scope := "subject"
	if u.Role != models.RoleTeacher {
		scope = "school"
	}
	// ?subject=ID preselects one subject, used by "announce to this class" on a roster.
	pre, _ := strconv.ParseInt(r.URL.Query().Get("subject"), 10, 64)
	for _, cs := range subjects {
		if cs.ID == pre {
			scope = "subject"
			break
		}
	}
	if scope != "subject" {
		pre = 0
	}
	a.render(w, r, http.StatusOK, "announcements/new", "New announcement",
		announceForm{Subjects: subjects, CanBroadcast: u.Role != models.RoleTeacher, Scope: scope, SubjectID: pre})
}

func (a *App) AnnouncementCreate(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	subjects, err := a.postableSubjects(r, u)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	f := announceForm{Subjects: subjects, CanBroadcast: u.Role != models.RoleTeacher,
		Title: r.PostFormValue("title"), Body: r.PostFormValue("body"),
		Scope: r.PostFormValue("scope"), SubjectID: formID(r, "class_subject_id")}
	showErr := func(msg string) {
		f.Error = msg
		a.render(w, r, http.StatusUnprocessableEntity, "announcements/new", "New announcement", f)
	}

	title, ok := text(f.Title, 3, 120)
	if !ok {
		showErr("Give the announcement a title of 3 to 120 characters.")
		return
	}
	body, ok := text(f.Body, 3, 4000)
	if !ok {
		showErr("Write the announcement (3 to 4000 characters).")
		return
	}

	n := repository.NewAnnouncement{AuthorID: u.ID, Title: title, Body: body, Scope: f.Scope}
	switch f.Scope {
	case "school", "staff":
		if u.Role == models.RoleTeacher {
			a.Forbidden(w, r) // teachers may only address their own subjects
			return
		}
	case "subject":
		allowed := false
		for _, cs := range subjects { // the list is already limited to what this user may address
			allowed = allowed || cs.ID == f.SubjectID
		}
		if !allowed {
			showErr("Choose one of your subjects.")
			return
		}
		n.ClassSubjectID = f.SubjectID
	default:
		showErr("Choose who should see this announcement.")
		return
	}

	if err := a.Store.CreateAnnouncement(r.Context(), n); err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.audit(r, u.ID, "announcement_posted", f.Scope)
	a.redirect(w, r, "/announcements", "ok", "Announcement posted.")
}

func (a *App) AnnouncementDelete(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	id, ok := pathID(r, "id")
	if !ok {
		a.NotFound(w, r)
		return
	}
	var onlyAuthor int64
	if u.Role == models.RoleTeacher {
		onlyAuthor = u.ID
	}
	err := a.Store.DeleteAnnouncement(r.Context(), id, onlyAuthor)
	if errors.Is(err, repository.ErrNotFound) {
		a.NotFound(w, r)
		return
	}
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.audit(r, u.ID, "announcement_deleted", r.PathValue("id"))
	a.redirect(w, r, "/announcements", "ok", "Announcement deleted.")
}
