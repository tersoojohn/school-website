package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"schoolportal/internal/middleware"
	"schoolportal/internal/models"
	"schoolportal/internal/repository"
	"schoolportal/internal/security"
	"schoolportal/internal/services"
)

type teacherSheet struct {
	repository.SheetProgress
	Summary services.Summary
}

type teacherDash struct {
	Term          *models.Term
	Sheets        []teacherSheet
	Announcements []models.Announcement
	TotalStudents int
	Classes       int
}

func (a *App) TeacherDashboard(w http.ResponseWriter, r *http.Request) {
	ctx, u := r.Context(), middleware.UserFrom(r.Context())
	d := teacherDash{}
	var err error
	roster, err := a.Store.TeacherRoster(ctx, u.ID)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	d.TotalStudents = len(roster)
	if d.Term, err = a.Store.CurrentTerm(ctx); err == nil {
		sheets, err := a.Store.SheetOverview(ctx, d.Term.ID, u.ID)
		if err != nil {
			a.ServerError(w, r, err)
			return
		}
		classes := map[string]bool{}
		for _, sh := range sheets {
			rows, err := a.Store.ListScoreSheet(ctx, d.Term.ID, sh.ClassSubjectID)
			if err != nil {
				a.ServerError(w, r, err)
				return
			}
			d.Sheets = append(d.Sheets, teacherSheet{SheetProgress: sh, Summary: services.Summarize(sheetTotals(rows))})
			classes[sh.ClassName] = true
		}
		d.Classes = len(classes)
	} else if !errors.Is(err, repository.ErrNotFound) {
		a.ServerError(w, r, err)
		return
	}
	if d.Announcements, err = a.Store.ListAnnouncements(ctx, u, 3); err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "teacher/dashboard", "My classes", d)
}

// ---- score sheet ----

type scoreRowView struct {
	StudentID int64
	Name      string
	Username  string
	CA, Exam  string
	Total     string
	Grade     string
	Locked    bool
}

type scoresView struct {
	CS           *models.ClassSubject
	Term         *models.Term
	Rows         []scoreRowView
	AllLocked    bool
	MaxCA        float64
	MaxExam      float64
	Errors       []string
	Summary      services.Summary
	Distribution []services.LabelCount
}

// ownedSheet loads the class/subject and the current term, and only proceeds when the
// signed-in teacher is the assigned teacher. Anyone else gets "not found".
func (a *App) ownedSheet(w http.ResponseWriter, r *http.Request) (*models.ClassSubject, *models.Term, bool) {
	u := middleware.UserFrom(r.Context())
	id, ok := pathID(r, "id")
	if !ok {
		a.NotFound(w, r)
		return nil, nil, false
	}
	cs, err := a.Store.GetClassSubject(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) || (err == nil && cs.TeacherID != u.ID) {
		a.NotFound(w, r)
		return nil, nil, false
	}
	if err != nil {
		a.ServerError(w, r, err)
		return nil, nil, false
	}
	term, err := a.Store.CurrentTerm(r.Context())
	if errors.Is(err, repository.ErrNotFound) {
		a.redirect(w, r, "/teacher", "error", "No term is active yet. Ask the principal or administration to set the current term.")
		return nil, nil, false
	}
	if err != nil {
		a.ServerError(w, r, err)
		return nil, nil, false
	}
	return cs, term, true
}

func buildRows(sheet []repository.ScoreRow, posted map[int64][2]string) ([]scoreRowView, bool) {
	rows := make([]scoreRowView, 0, len(sheet))
	allLocked := len(sheet) > 0
	for _, s := range sheet {
		row := scoreRowView{StudentID: s.StudentID, Name: s.FullName, Username: s.Username, Locked: s.Status == "submitted"}
		if p, ok := posted[s.StudentID]; ok && !row.Locked {
			row.CA, row.Exam = p[0], p[1]
		} else {
			row.CA, row.Exam = scoreStr(s.CA), scoreStr(s.Exam)
		}
		if s.CA != nil && s.Exam != nil && row.CA == scoreStr(s.CA) && row.Exam == scoreStr(s.Exam) {
			total := *s.CA + *s.Exam
			row.Total = strconv.FormatFloat(total, 'f', -1, 64)
			row.Grade, _ = services.Grade(total)
		}
		allLocked = allLocked && row.Locked
		rows = append(rows, row)
	}
	return rows, allLocked
}

func scoreStr(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func (a *App) showSheet(w http.ResponseWriter, r *http.Request, status int, cs *models.ClassSubject, term *models.Term, posted map[int64][2]string, errs []string) {
	sheet, err := a.Store.ListScoreSheet(r.Context(), term.ID, cs.ID)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	rows, locked := buildRows(sheet, posted)
	totals := sheetTotals(sheet)
	a.render(w, r, status, "teacher/scores", cs.Label(), scoresView{
		CS: cs, Term: term, Rows: rows, AllLocked: locked, MaxCA: services.MaxCA, MaxExam: services.MaxExam, Errors: errs,
		Summary: services.Summarize(totals), Distribution: services.Distribution(totals)})
}

func (a *App) ScoresForm(w http.ResponseWriter, r *http.Request) {
	cs, term, ok := a.ownedSheet(w, r)
	if !ok {
		return
	}
	a.showSheet(w, r, http.StatusOK, cs, term, nil, nil)
}

func (a *App) ScoresSave(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	cs, term, ok := a.ownedSheet(w, r)
	if !ok {
		return
	}
	// The student list always comes from the database, never from the posted form,
	// so a teacher cannot write scores for students outside this class.
	sheet, err := a.Store.ListScoreSheet(r.Context(), term.ID, cs.ID)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	var (
		inputs []repository.ScoreInput
		posted = map[int64][2]string{}
		errs   []string
	)
	for _, row := range sheet {
		if row.Status == "submitted" {
			continue
		}
		id := strconv.FormatInt(row.StudentID, 10)
		rawCA, rawEx := r.PostFormValue("ca_"+id), r.PostFormValue("exam_"+id)
		posted[row.StudentID] = [2]string{rawCA, rawEx}
		ca, err1 := services.ParseScore(rawCA, services.MaxCA)
		ex, err2 := services.ParseScore(rawEx, services.MaxExam)
		if err1 != nil {
			errs = append(errs, fmt.Sprintf("%s, continuous assessment: %s.", row.FullName, err1))
		}
		if err2 != nil {
			errs = append(errs, fmt.Sprintf("%s, exam: %s.", row.FullName, err2))
		}
		inputs = append(inputs, repository.ScoreInput{StudentID: row.StudentID, CA: ca, Exam: ex})
	}
	if len(errs) > 0 {
		a.showSheet(w, r, http.StatusUnprocessableEntity, cs, term, posted, errs)
		return
	}
	if len(inputs) == 0 {
		a.redirect(w, r, r.URL.Path, "error", "This sheet is already submitted. Ask the principal to reopen it if a score must change.")
		return
	}
	if err := a.Store.SaveScores(r.Context(), term.ID, cs.ID, u.ID, inputs); err != nil {
		a.ServerError(w, r, err)
		return
	}

	if r.PostFormValue("action") == "submit" {
		err := a.Store.SubmitSheet(r.Context(), term.ID, cs.ID)
		if errors.Is(err, repository.ErrIncomplete) {
			a.redirect(w, r, r.URL.Path, "error", "Scores were saved, but the sheet was not submitted: every student needs both a continuous assessment and an exam score.")
			return
		}
		if err != nil {
			a.ServerError(w, r, err)
			return
		}
		a.audit(r, u.ID, "results_submitted", cs.Label()+", "+term.Name)
		a.redirect(w, r, "/teacher", "ok", cs.Label()+" submitted. Students can now see their results.")
		return
	}
	a.audit(r, u.ID, "results_saved", cs.Label()+", "+term.Name)
	a.redirect(w, r, r.URL.Path, "ok", "Scores saved as a draft. Students cannot see them until you submit.")
}

// ---- bank details ----

type bankView struct {
	Bank  *models.BankDetails
	Error string
	Form  map[string]string
}

func (a *App) BankForm(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	b, err := a.Store.GetBank(r.Context(), u.ID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		a.ServerError(w, r, err)
		return
	}
	form := map[string]string{}
	if b != nil {
		b.AccountNumber = "" // the teacher page only ever shows the last four digits
		form["bank_name"], form["account_name"] = b.BankName, b.AccountName
	}
	a.render(w, r, http.StatusOK, "teacher/bank", "Bank details", bankView{Bank: b, Form: form})
}

func (a *App) BankSave(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	form := map[string]string{"bank_name": r.PostFormValue("bank_name"), "account_name": r.PostFormValue("account_name")}
	existing, _ := a.Store.GetBank(r.Context(), u.ID)
	if existing != nil {
		existing.AccountNumber = ""
	}
	showErr := func(msg string) {
		a.render(w, r, http.StatusUnprocessableEntity, "teacher/bank", "Bank details", bankView{Bank: existing, Form: form, Error: msg})
	}
	bank, ok := text(form["bank_name"], 2, 60)
	if !ok {
		showErr("Enter the name of your bank.")
		return
	}
	holder, ok := text(form["account_name"], 2, 100)
	if !ok {
		showErr("Enter the account name exactly as the bank has it.")
		return
	}
	number := r.PostFormValue("account_number")
	if !accountRe.MatchString(number) {
		showErr("The account number must be exactly 10 digits.")
		return
	}
	enc, err := security.Encrypt(a.Cfg.DataKey, number, bankAAD(u.ID))
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	if err := a.Store.SaveBank(r.Context(), u.ID, bank, holder, enc, number[len(number)-4:]); err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.audit(r, u.ID, "bank_details_updated", "")
	a.redirect(w, r, "/teacher/bank", "ok", "Bank details saved.")
}

func bankAAD(userID int64) string { return "bank:" + strconv.FormatInt(userID, 10) }

// loadBank returns bank details with the account number decrypted. Only call it for
// principal/administrator views, and record the access in the audit log.
func (a *App) loadBank(r *http.Request, userID int64) (*models.BankDetails, error) {
	b, err := a.Store.GetBank(r.Context(), userID)
	if err != nil {
		return nil, err
	}
	plain, err := security.Decrypt(a.Cfg.DataKey, b.AccountNumber, bankAAD(userID))
	if err != nil {
		return nil, err
	}
	b.AccountNumber = plain
	return b, nil
}

// sheetTotals returns the total of every student who has both scores recorded.
func sheetTotals(sheet []repository.ScoreRow) []float64 {
	var out []float64
	for _, s := range sheet {
		if s.CA != nil && s.Exam != nil {
			out = append(out, *s.CA+*s.Exam)
		}
	}
	return out
}
