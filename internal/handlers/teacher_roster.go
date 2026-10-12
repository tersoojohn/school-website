package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"schoolportal/internal/middleware"
	"schoolportal/internal/repository"
	"schoolportal/internal/services"
)

type rosterByClass struct {
	Class string
	Count int
}

type teacherStudentsView struct {
	Students []repository.RosterEntry
	Total    int
	ByClass  []rosterByClass
	Subjects int
}

// TeacherStudents is the one-click list of every student the teacher is assigned.
func (a *App) TeacherStudents(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	roster, err := a.Store.TeacherRoster(r.Context(), u.ID)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	cs, err := a.Store.ListClassSubjects(r.Context(), u.ID)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	v := teacherStudentsView{Students: roster, Total: len(roster), Subjects: len(cs)}
	idx := map[string]int{}
	for _, e := range roster {
		i, ok := idx[e.Class]
		if !ok {
			i = len(v.ByClass)
			idx[e.Class] = i
			v.ByClass = append(v.ByClass, rosterByClass{Class: e.Class})
		}
		v.ByClass[i].Count++
	}
	a.render(w, r, http.StatusOK, "teacher/students", "All my students", v)
}

// csvSafe stops spreadsheet programs from running a cell as a formula.
func csvSafe(s string) string {
	if s != "" && strings.ContainsAny(s[:1], "=+-@\t\r") {
		return "'" + s
	}
	return s
}

var nonFile = regexp.MustCompile(`[^A-Za-z0-9]+`)

func (a *App) sendCSV(w http.ResponseWriter, name string, rows [][]string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.Trim(nonFile.ReplaceAllString(name, "-"), "-")+`.csv"`)
	cw := csv.NewWriter(w)
	for _, row := range rows {
		for i := range row {
			row[i] = csvSafe(row[i])
		}
		_ = cw.Write(row)
	}
	cw.Flush()
}

func (a *App) TeacherStudentsCSV(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	roster, err := a.Store.TeacherRoster(r.Context(), u.ID)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	rows := [][]string{{"Name", "Student ID", "Class", "Subjects"}}
	for _, e := range roster {
		rows = append(rows, []string{e.FullName, e.Username, e.Class, e.Subjects})
	}
	a.audit(r, u.ID, "roster_exported", strconv.Itoa(len(roster))+" students")
	a.sendCSV(w, "my-students", rows)
}

func (a *App) ScoresCSV(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	cs, term, ok := a.ownedSheet(w, r)
	if !ok {
		return
	}
	sheet, err := a.Store.ListScoreSheet(r.Context(), term.ID, cs.ID)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	rows := [][]string{{"Name", "Student ID", "CA", "Exam", "Total", "Grade", "Status"}}
	for _, s := range sheet {
		row := []string{s.FullName, s.Username, scoreStr(s.CA), scoreStr(s.Exam), "", "", s.Status}
		if s.CA != nil && s.Exam != nil {
			t := *s.CA + *s.Exam
			g, _ := services.Grade(t)
			row[4], row[5] = strconv.FormatFloat(t, 'f', -1, 64), g
		}
		rows = append(rows, row)
	}
	a.audit(r, u.ID, "scores_exported", cs.Label())
	a.sendCSV(w, fmt.Sprintf("%s %s %s", cs.SubjectName, cs.ClassName, term.Name), rows)
}
