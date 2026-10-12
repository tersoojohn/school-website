package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"schoolportal/internal/middleware"
	"schoolportal/internal/models"
	"schoolportal/internal/repository"
	"schoolportal/internal/security"
)

func newUnusablePassword() (string, error) { return security.UnusablePassword() }

// ---- students ----

type studentsView struct {
	Students []models.Student
	Classes  []models.Class
	ClassID  int64
}

func (a *App) Students(w http.ResponseWriter, r *http.Request) {
	classID, _ := strconv.ParseInt(r.URL.Query().Get("class"), 10, 64)
	list, err := a.Store.ListStudents(r.Context(), classID)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	classes, err := a.Store.ListClasses(r.Context())
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "manage/students", "Students", studentsView{Students: list, Classes: classes, ClassID: classID})
}

type studentFormView struct {
	Classes []models.Class
	Error   string
	Form    map[string]string
	ClassID int64
}

func (a *App) StudentForm(w http.ResponseWriter, r *http.Request) {
	classes, err := a.Store.ListClasses(r.Context())
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "manage/student_new", "Register a student", studentFormView{Classes: classes, Form: map[string]string{}})
}

func (a *App) StudentCreate(w http.ResponseWriter, r *http.Request) {
	ctx, actor := r.Context(), middleware.UserFrom(r.Context())
	classes, err := a.Store.ListClasses(ctx)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	form := map[string]string{
		"full_name": r.PostFormValue("full_name"), "date_of_birth": r.PostFormValue("date_of_birth"),
		"gender": r.PostFormValue("gender"), "guardian_name": r.PostFormValue("guardian_name"),
		"guardian_phone": r.PostFormValue("guardian_phone"),
	}
	classID := formID(r, "class_id")
	showErr := func(msg string) {
		a.render(w, r, http.StatusUnprocessableEntity, "manage/student_new", "Register a student",
			studentFormView{Classes: classes, Error: msg, Form: form, ClassID: classID})
	}

	name, ok := text(form["full_name"], 2, 100)
	if !ok {
		showErr("Enter the student's full name (2 to 100 characters).")
		return
	}
	if exists, err := a.Store.ClassExists(ctx, classID); err != nil || !exists {
		showErr("Choose a class.")
		return
	}
	dob, ok := optionalDate(form["date_of_birth"])
	if !ok {
		showErr("Enter the date of birth as a past date, or leave it empty.")
		return
	}
	gender := form["gender"]
	if gender != "" && gender != "Female" && gender != "Male" {
		showErr("Choose a gender from the list, or leave it empty.")
		return
	}
	guardian, ok := text(form["guardian_name"], 0, 100)
	if !ok {
		showErr("The guardian's name is too long.")
		return
	}
	gphone, ok := optionalPhone(form["guardian_phone"])
	if !ok {
		showErr("Enter a valid guardian phone number or leave it empty.")
		return
	}

	hash, err := newUnusablePassword()
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	created, err := a.Store.CreateStudent(ctx,
		repository.NewUser{FullName: name, PasswordHash: hash, CreatedBy: actor.ID},
		repository.NewStudent{ClassID: classID, GuardianName: guardian, GuardianPhone: gphone, DateOfBirth: dob, Gender: gender})
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.audit(r, actor.ID, "student_registered", created.Username)
	a.showSetupLink(w, r, created.ID, name, created.Username, models.RoleStudent, "/manage/students/new", "Register another student")
}

// ---- staff ----

type staffView struct {
	Staff []models.Staff
	Kind  string
}

func (a *App) StaffList(w http.ResponseWriter, r *http.Request) {
	kind := models.StaffKind(r.URL.Query().Get("kind"))
	if kind != models.StaffEducating && kind != models.StaffNonEducating {
		kind = ""
	}
	list, err := a.Store.ListStaff(r.Context(), kind)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "manage/staff", "Staff", staffView{Staff: list, Kind: string(kind)})
}

type staffFormView struct {
	Error string
	Form  map[string]string
}

func (a *App) StaffForm(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusOK, "manage/staff_new", "Register staff", staffFormView{Form: map[string]string{"kind": "educating"}})
}

func (a *App) StaffCreate(w http.ResponseWriter, r *http.Request) {
	ctx, actor := r.Context(), middleware.UserFrom(r.Context())
	form := map[string]string{
		"full_name": r.PostFormValue("full_name"), "phone": r.PostFormValue("phone"),
		"kind": r.PostFormValue("kind"), "job_title": r.PostFormValue("job_title"),
	}
	showErr := func(msg string) {
		a.render(w, r, http.StatusUnprocessableEntity, "manage/staff_new", "Register staff", staffFormView{Error: msg, Form: form})
	}
	name, ok := text(form["full_name"], 2, 100)
	if !ok {
		showErr("Enter the staff member's full name (2 to 100 characters).")
		return
	}
	phone, ok := optionalPhone(form["phone"])
	if !ok {
		showErr("Enter a valid phone number or leave it empty.")
		return
	}
	kind := models.StaffKind(form["kind"])
	if kind != models.StaffEducating && kind != models.StaffNonEducating {
		showErr("Choose whether this person is educating or non-educating staff.")
		return
	}
	minTitle := 0
	if kind == models.StaffNonEducating {
		minTitle = 2
	}
	title, ok := text(form["job_title"], minTitle, 60)
	if !ok {
		showErr("Enter the job title, for example Cleaner or Security.")
		return
	}

	hash, err := newUnusablePassword()
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	created, err := a.Store.CreateStaff(ctx, repository.NewUser{FullName: name, Phone: phone, PasswordHash: hash, CreatedBy: actor.ID}, kind, title)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	role := models.RoleTeacher
	if kind == models.StaffNonEducating {
		role = models.RoleSupport
	}
	a.audit(r, actor.ID, "staff_registered", created.Username)
	a.showSetupLink(w, r, created.ID, name, created.Username, role, "/manage/staff/new", "Register another staff member")
}

// ---- one person ----

type personView struct {
	Person  *models.User
	Student *models.Student
	Staff   *models.Staff
	Bank    *models.BankDetails
	NoBank  bool
}

// loadManaged loads a user and verifies the signed-in actor is allowed to manage that role.
// Anything else is reported as "not found" so account ids cannot be probed.
func (a *App) loadManaged(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	actor := middleware.UserFrom(r.Context())
	id, ok := pathID(r, "id")
	if !ok {
		a.NotFound(w, r)
		return nil, false
	}
	target, err := a.Store.GetUser(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) || (err == nil && !actor.Role.CanManage(target.Role)) {
		a.NotFound(w, r)
		return nil, false
	}
	if err != nil {
		a.ServerError(w, r, err)
		return nil, false
	}
	return target, true
}

func (a *App) Person(w http.ResponseWriter, r *http.Request) {
	actor := middleware.UserFrom(r.Context())
	target, ok := a.loadManaged(w, r)
	if !ok {
		return
	}
	v := personView{Person: target}
	var err error
	switch target.Role {
	case models.RoleStudent:
		v.Student, err = a.Store.GetStudent(r.Context(), target.ID)
	case models.RoleTeacher, models.RoleSupport:
		v.Staff, err = a.Store.GetStaff(r.Context(), target.ID)
	}
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	if target.Role == models.RoleTeacher {
		v.Bank, err = a.loadBank(r, target.ID)
		if errors.Is(err, repository.ErrNotFound) {
			v.NoBank, err = true, nil
		}
		if err != nil {
			a.ServerError(w, r, err)
			return
		}
		if v.Bank != nil {
			a.audit(r, actor.ID, "bank_details_viewed", target.Username)
		}
	}
	a.render(w, r, http.StatusOK, "manage/person", target.FullName, v)
}

func (a *App) ResetPassword(w http.ResponseWriter, r *http.Request) {
	actor := middleware.UserFrom(r.Context())
	target, ok := a.loadManaged(w, r)
	if !ok {
		return
	}
	hash, err := newUnusablePassword()
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	// The old password stops working now; the person sets a new one from the link.
	if err := a.Store.MarkPendingSetup(r.Context(), target.ID, hash); err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.audit(r, actor.ID, "setup_link_issued", target.Username)
	a.showSetupLink(w, r, target.ID, target.FullName, target.Username, target.Role,
		"/manage/people/"+strconv.FormatInt(target.ID, 10), "Back to account")
}

func (a *App) SetActive(w http.ResponseWriter, r *http.Request) {
	actor := middleware.UserFrom(r.Context())
	target, ok := a.loadManaged(w, r)
	if !ok {
		return
	}
	active := r.PostFormValue("active") == "1"
	if err := a.Store.SetActive(r.Context(), target.ID, active); err != nil {
		a.ServerError(w, r, err)
		return
	}
	msg, action := target.FullName+" can sign in again.", "account_activated"
	if !active {
		_ = a.Store.DeleteUserSessions(r.Context(), target.ID)
		msg, action = target.FullName+" was deactivated and signed out everywhere.", "account_deactivated"
	}
	a.audit(r, actor.ID, action, target.Username)
	a.redirect(w, r, "/manage/people/"+strconv.FormatInt(target.ID, 10), "ok", msg)
}
