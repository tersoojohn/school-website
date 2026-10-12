package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"schoolportal/internal/models"
	"schoolportal/internal/repository"
	"schoolportal/internal/services"
)

type analyticsView struct {
	Terms        []models.Term
	Term         *models.Term
	Summary      services.Summary
	Distribution []services.LabelCount
	Subjects     []repository.SubjectStat
	Classes      []repository.ClassStat
	Trend        []repository.TermStat
	Enrollment   []repository.Count
	Insights     []string
	PassMark     float64
}

// Analytics is the principal's school-wide performance page for one term.
func (a *App) Analytics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	terms, err := a.Store.ListTerms(ctx)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	v := analyticsView{Terms: terms, PassMark: services.PassMark}

	want, _ := strconv.ParseInt(r.URL.Query().Get("term"), 10, 64)
	for i := range terms {
		if terms[i].ID == want || (want == 0 && terms[i].IsCurrent) {
			v.Term = &terms[i]
			break
		}
	}
	if v.Term == nil && len(terms) > 0 {
		v.Term = &terms[0]
	}

	if v.Enrollment, err = a.Store.Enrollment(ctx); err == nil {
		v.Trend, err = a.Store.TermTrend(ctx)
	}
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	if v.Term != nil {
		totals, err := a.Store.Totals(ctx, v.Term.ID, 0, 0)
		if err != nil {
			a.ServerError(w, r, err)
			return
		}
		v.Summary = services.Summarize(totals)
		v.Distribution = services.Distribution(totals)
		if v.Subjects, err = a.Store.SubjectStats(ctx, v.Term.ID); err == nil {
			v.Classes, err = a.Store.ClassStats(ctx, v.Term.ID)
		}
		if err != nil {
			a.ServerError(w, r, err)
			return
		}
		v.Insights = insights(v)
	}
	a.render(w, r, http.StatusOK, "principal/analytics", "Analytics", v)
}

// insights turns the numbers into a few plain sentences, so the page answers "what stands out?".
func insights(v analyticsView) []string {
	var out []string
	if v.Summary.Count == 0 {
		return out
	}
	out = append(out, fmt.Sprintf("%s of results this term are passes, with a school average of %s.",
		pct(v.Summary.PassRate), f1(v.Summary.Mean)))
	if n := len(v.Subjects); n >= 2 {
		best, worst := v.Subjects[0], v.Subjects[n-1]
		out = append(out, fmt.Sprintf("%s is the strongest subject (average %s). %s is the weakest (average %s, %s pass rate).",
			best.Subject, f1(best.Mean), worst.Subject, f1(worst.Mean), pct(worst.PassRate)))
	}
	if n := len(v.Classes); n >= 2 {
		hi, lo := v.Classes[0], v.Classes[0]
		for _, c := range v.Classes {
			if c.Mean > hi.Mean {
				hi = c
			}
			if c.Mean < lo.Mean {
				lo = c
			}
		}
		out = append(out, fmt.Sprintf("%s has the highest class average (%s). %s has the lowest (%s).", hi.Class, f1(hi.Mean), lo.Class, f1(lo.Mean)))
	}
	if n := len(v.Trend); n >= 2 {
		d := v.Trend[n-1].Mean - v.Trend[n-2].Mean
		dir := "up"
		if d < 0 {
			dir, d = "down", -d
		}
		out = append(out, fmt.Sprintf("The school average is %s %s points compared with the previous term.", dir, f1(services.Round1(d))))
	}
	return out
}

func f1(v float64) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	if len(s) > 2 && s[len(s)-2:] == ".0" {
		return s[:len(s)-2]
	}
	return s
}

func pct(v float64) string { return f1(v) + "%" }

// termForDashboard returns the current term or nil when none is set.
func (a *App) currentTermOrNil(r *http.Request) (*models.Term, error) {
	t, err := a.Store.CurrentTerm(r.Context())
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil
	}
	return t, err
}
