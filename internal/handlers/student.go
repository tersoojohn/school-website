package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"schoolportal/internal/middleware"
	"schoolportal/internal/models"
	"schoolportal/internal/repository"
	"schoolportal/internal/services"
)

type studentRow struct {
	services.ReportRow
	ClassAvg float64
	HasAvg   bool
}

type studentDash struct {
	Student       *models.Student
	Term          *models.Term
	Report        services.Report
	Rows          []studentRow
	Highlights    []string
	Announcements []models.Announcement
}

// minClassSample is the fewest results a class average needs before it is shown to students.
const minClassSample = 3

func (a *App) studentRows(r *http.Request, classID, termID int64, rep services.Report) ([]studentRow, error) {
	avgs, err := a.Store.ClassAverages(r.Context(), classID, termID, minClassSample)
	if err != nil {
		return nil, err
	}
	rows := make([]studentRow, 0, len(rep.Rows))
	for _, rr := range rep.Rows {
		sr := studentRow{ReportRow: rr}
		if av, ok := avgs[rr.Subject]; ok {
			sr.ClassAvg, sr.HasAvg = av.Mean, true
		}
		rows = append(rows, sr)
	}
	return rows, nil
}

func highlights(rep services.Report) []string {
	if len(rep.Rows) < 2 {
		return nil
	}
	best, low := rep.Rows[0], rep.Rows[0]
	for _, r := range rep.Rows {
		if r.Total > best.Total {
			best = r
		}
		if r.Total < low.Total {
			low = r
		}
	}
	out := []string{best.Subject + " is your strongest subject this term at " + f1(best.Total) + "."}
	if low.Subject != best.Subject {
		out = append(out, low.Subject+" has the most room to grow at "+f1(low.Total)+".")
	}
	return out
}

func (a *App) StudentDashboard(w http.ResponseWriter, r *http.Request) {
	ctx, u := r.Context(), middleware.UserFrom(r.Context())
	st, err := a.Store.GetStudent(ctx, u.ID)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	d := studentDash{Student: st}
	if d.Term, err = a.Store.CurrentTerm(ctx); err == nil {
		raw, err := a.Store.StudentResults(ctx, u.ID, d.Term.ID)
		if err != nil {
			a.ServerError(w, r, err)
			return
		}
		d.Report = services.BuildReport(raw)
		if d.Rows, err = a.studentRows(r, st.ClassID, d.Term.ID, d.Report); err != nil {
			a.ServerError(w, r, err)
			return
		}
		d.Highlights = highlights(d.Report)
	} else if !errors.Is(err, repository.ErrNotFound) {
		a.ServerError(w, r, err)
		return
	}
	if d.Announcements, err = a.Store.ListAnnouncements(ctx, u, 4); err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "student/dashboard", "My report", d)
}

type studentHistory struct {
	Terms    []models.Term
	Selected *models.Term
	Report   services.Report
	Rows     []studentRow
	Trend    []repository.TermStat
}

func (a *App) StudentResults(w http.ResponseWriter, r *http.Request) {
	ctx, u := r.Context(), middleware.UserFrom(r.Context())
	st, err := a.Store.GetStudent(ctx, u.ID)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	terms, err := a.Store.StudentTerms(ctx, u.ID)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	v := studentHistory{Terms: terms}
	if v.Trend, err = a.Store.StudentTermAverages(ctx, u.ID); err != nil {
		a.ServerError(w, r, err)
		return
	}
	want, _ := strconv.ParseInt(r.URL.Query().Get("term"), 10, 64)
	for i := range terms {
		if terms[i].ID == want {
			v.Selected = &terms[i]
			break
		}
	}
	if v.Selected == nil && len(terms) > 0 {
		v.Selected = &terms[0]
	}
	if v.Selected != nil {
		raw, err := a.Store.StudentResults(ctx, u.ID, v.Selected.ID)
		if err != nil {
			a.ServerError(w, r, err)
			return
		}
		v.Report = services.BuildReport(raw)
		if v.Rows, err = a.studentRows(r, st.ClassID, v.Selected.ID, v.Report); err != nil {
			a.ServerError(w, r, err)
			return
		}
	}
	a.render(w, r, http.StatusOK, "student/results", "Past terms", v)
}
