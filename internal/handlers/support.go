package handlers

import (
	"net/http"

	"schoolportal/internal/middleware"
	"schoolportal/internal/models"
)

type supportDash struct {
	Staff         *models.Staff
	Announcements []models.Announcement
}

func (a *App) SupportDashboard(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	staff, err := a.Store.GetStaff(r.Context(), u.ID)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	ann, err := a.Store.ListAnnouncements(r.Context(), u, 8)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "support/dashboard", "My panel", supportDash{Staff: staff, Announcements: ann})
}
