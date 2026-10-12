package handlers

import (
	"net/http"

	"schoolportal/internal/middleware"
	"schoolportal/internal/models"
	"schoolportal/internal/repository"
)

type adminDash struct {
	Counts        headcount
	Announcements []models.Announcement
	Enrollment    []repository.Count
	Registrations []repository.Count
	Pending       []models.User
}

func (a *App) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	counts, err := a.Store.CountActiveByRole(r.Context())
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	d := adminDash{Counts: toHeadcount(counts)}
	if d.Announcements, err = a.Store.ListAnnouncements(r.Context(), u, 3); err == nil {
		if d.Enrollment, err = a.Store.Enrollment(r.Context()); err == nil {
			if d.Registrations, err = a.Store.RegistrationsByMonth(r.Context(), 6); err == nil {
				d.Pending, err = a.Store.ListPendingSetups(r.Context(), 8)
			}
		}
	}
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "admin/dashboard", "Overview", d)
}

// headcount is the number of active accounts per role, shaped for templates.
type headcount struct{ Students, Teachers, Support, Admins int }

func toHeadcount(m map[models.Role]int) headcount {
	return headcount{Students: m[models.RoleStudent], Teachers: m[models.RoleTeacher],
		Support: m[models.RoleSupport], Admins: m[models.RoleAdmin]}
}
