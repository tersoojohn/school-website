package handlers

import (
	"errors"
	"net/http"
	"strings"

	"schoolportal/internal/middleware"
	"schoolportal/internal/models"
	"schoolportal/internal/repository"
)

type academicsView struct {
	Classes     []models.Class
	Subjects    []models.Subject
	Terms       []models.Term
	Assignments []models.ClassSubject
	Teachers    []models.Staff
}

func (a *App) Academics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var v academicsView
	var err error
	if v.Classes, err = a.Store.ListClasses(ctx); err == nil {
		if v.Subjects, err = a.Store.ListSubjects(ctx); err == nil {
			if v.Terms, err = a.Store.ListTerms(ctx); err == nil {
				if v.Assignments, err = a.Store.ListClassSubjects(ctx, 0); err == nil {
					var staff []models.Staff
					if staff, err = a.Store.ListStaff(ctx, models.StaffEducating); err == nil {
						for _, s := range staff {
							if s.Active {
								v.Teachers = append(v.Teachers, s)
							}
						}
					}
				}
			}
		}
	}
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "manage/academics", "Classes and subjects", v)
}

func (a *App) academicsResult(w http.ResponseWriter, r *http.Request, err error, okMsg, action, detail string) {
	switch {
	case errors.Is(err, repository.ErrConflict):
		a.redirect(w, r, "/manage/academics", "error", "That already exists.")
	case err != nil:
		a.ServerError(w, r, err)
	default:
		a.audit(r, middleware.UserFrom(r.Context()).ID, action, detail)
		a.redirect(w, r, "/manage/academics", "ok", okMsg)
	}
}

func (a *App) ClassCreate(w http.ResponseWriter, r *http.Request) {
	name, ok := text(r.PostFormValue("name"), 1, 40)
	if !ok {
		a.redirect(w, r, "/manage/academics", "error", "Enter a class name of up to 40 characters.")
		return
	}
	a.academicsResult(w, r, a.Store.CreateClass(r.Context(), name), "Class added.", "class_created", name)
}

func (a *App) SubjectCreate(w http.ResponseWriter, r *http.Request) {
	name, okName := text(r.PostFormValue("name"), 1, 60)
	code := strings.ToUpper(strings.TrimSpace(r.PostFormValue("code")))
	if !okName || !codeRe.MatchString(code) {
		a.redirect(w, r, "/manage/academics", "error", "Enter a subject name and a short code of 2 to 12 letters or numbers.")
		return
	}
	a.academicsResult(w, r, a.Store.CreateSubject(r.Context(), name, code), "Subject added.", "subject_created", name)
}

func (a *App) TermCreate(w http.ResponseWriter, r *http.Request) {
	name, ok := text(r.PostFormValue("name"), 3, 60)
	if !ok {
		a.redirect(w, r, "/manage/academics", "error", "Enter a term name such as 2026/2027 First Term.")
		return
	}
	a.academicsResult(w, r, a.Store.CreateTerm(r.Context(), name), "Term added.", "term_created", name)
}

func (a *App) TermSetCurrent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		a.NotFound(w, r)
		return
	}
	err := a.Store.SetCurrentTerm(r.Context(), id)
	if err != nil {
		a.NotFound(w, r)
		return
	}
	a.academicsResult(w, r, nil, "Current term updated. Teachers now record scores for it.", "term_set_current", r.PathValue("id"))
}

func (a *App) Assign(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	classID, subjectID, teacherID := formID(r, "class_id"), formID(r, "subject_id"), formID(r, "teacher_id")
	if ok, err := a.Store.ClassExists(ctx, classID); err != nil || !ok {
		a.redirect(w, r, "/manage/academics", "error", "Choose a class.")
		return
	}
	subjects, err := a.Store.ListSubjects(ctx)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	found := false
	for _, s := range subjects {
		found = found || s.ID == subjectID
	}
	if !found {
		a.redirect(w, r, "/manage/academics", "error", "Choose a subject.")
		return
	}
	if teacherID != 0 {
		if ok, err := a.Store.IsActiveTeacher(ctx, teacherID); err != nil || !ok {
			a.redirect(w, r, "/manage/academics", "error", "Choose a teacher from the list.")
			return
		}
	}
	err = a.Store.AssignClassSubject(ctx, classID, subjectID, teacherID)
	a.academicsResult(w, r, err, "Assignment saved.", "subject_assigned", r.PostFormValue("class_id")+"/"+r.PostFormValue("subject_id"))
}
