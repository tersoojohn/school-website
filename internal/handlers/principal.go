package handlers

import (
	"errors"
	"net/http"

	"schoolportal/internal/middleware"
	"schoolportal/internal/models"
	"schoolportal/internal/repository"
	"schoolportal/internal/services"
)

type principalDash struct {
	Counts        headcount
	Term          *models.Term
	SheetsTotal   int
	SheetsDone    int
	Outstanding   []repository.SheetProgress
	Announcements []models.Announcement
	Audit         []repository.AuditEntry
	Enrollment    []repository.Count
	Distribution  []services.LabelCount
	Summary       services.Summary
	Pending       []models.User
}

func (a *App) PrincipalDashboard(w http.ResponseWriter, r *http.Request) {
	ctx, u := r.Context(), middleware.UserFrom(r.Context())
	d := principalDash{}
	counts, err := a.Store.CountActiveByRole(ctx)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	d.Counts = toHeadcount(counts)
	if d.Term, err = a.Store.CurrentTerm(ctx); err == nil {
		sheets, err := a.Store.SheetOverview(ctx, d.Term.ID, 0)
		if err != nil {
			a.ServerError(w, r, err)
			return
		}
		d.SheetsTotal = len(sheets)
		for _, s := range sheets {
			if s.IsSubmitted() {
				d.SheetsDone++
			} else if len(d.Outstanding) < 6 {
				d.Outstanding = append(d.Outstanding, s)
			}
		}
	} else if !errors.Is(err, repository.ErrNotFound) {
		a.ServerError(w, r, err)
		return
	}
	if d.Enrollment, err = a.Store.Enrollment(ctx); err != nil {
		a.ServerError(w, r, err)
		return
	}
	if d.Term != nil {
		totals, err := a.Store.Totals(ctx, d.Term.ID, 0, 0)
		if err != nil {
			a.ServerError(w, r, err)
			return
		}
		d.Summary, d.Distribution = services.Summarize(totals), services.Distribution(totals)
	}
	if d.Pending, err = a.Store.ListPendingSetups(ctx, 6); err != nil {
		a.ServerError(w, r, err)
		return
	}
	if d.Announcements, err = a.Store.ListAnnouncements(ctx, u, 3); err != nil {
		a.ServerError(w, r, err)
		return
	}
	if d.Audit, err = a.Store.RecentAudit(ctx, 10); err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "principal/dashboard", "Overview", d)
}

type resultsOverview struct {
	Term   *models.Term
	Sheets []repository.SheetProgress
}

func (a *App) ResultsOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	term, err := a.Store.CurrentTerm(ctx)
	if errors.Is(err, repository.ErrNotFound) {
		a.render(w, r, http.StatusOK, "principal/results", "Results progress", resultsOverview{})
		return
	}
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	sheets, err := a.Store.SheetOverview(ctx, term.ID, 0)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "principal/results", "Results progress", resultsOverview{Term: term, Sheets: sheets})
}

// ReopenSheet lets the principal send a submitted score sheet back to the teacher for correction.
func (a *App) ReopenSheet(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	id, ok := pathID(r, "id")
	if !ok {
		a.NotFound(w, r)
		return
	}
	term, err := a.Store.CurrentTerm(r.Context())
	if err != nil {
		a.redirect(w, r, "/principal/results", "error", "There is no current term.")
		return
	}
	cs, err := a.Store.GetClassSubject(r.Context(), id)
	if err != nil {
		a.NotFound(w, r)
		return
	}
	if err := a.Store.ReopenSheet(r.Context(), term.ID, id); err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.audit(r, u.ID, "results_reopened", cs.Label()+", "+term.Name)
	a.redirect(w, r, "/principal/results", "ok", cs.Label()+" was returned to the teacher for correction.")
}

type adminsView struct {
	Admins []models.User
	Error  string
	Name   string
	Phone  string
}

func (a *App) Admins(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListAdmins(r.Context())
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "principal/admins", "Administrators", adminsView{Admins: list})
}

func (a *App) AdminCreate(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	name, okName := text(r.PostFormValue("full_name"), 2, 100)
	phone, okPhone := optionalPhone(r.PostFormValue("phone"))
	showErr := func(msg string) {
		list, _ := a.Store.ListAdmins(r.Context())
		a.render(w, r, http.StatusUnprocessableEntity, "principal/admins", "Administrators",
			adminsView{Admins: list, Error: msg, Name: name, Phone: phone})
	}
	if !okName {
		showErr("Enter the administrator's full name (2 to 100 characters).")
		return
	}
	if !okPhone {
		showErr("Enter a valid phone number or leave it empty.")
		return
	}
	hash, err := newUnusablePassword()
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	created, err := a.Store.CreateAdmin(r.Context(), repository.NewUser{FullName: name, Phone: phone, PasswordHash: hash, CreatedBy: u.ID})
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.audit(r, u.ID, "admin_created", created.Username)
	a.showSetupLink(w, r, created.ID, name, created.Username, models.RoleAdmin, "/principal/admins", "Back to administrators")
}
