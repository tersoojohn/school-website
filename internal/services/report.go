package services

import "math"

// RawResult is one submitted subject score as stored.
type RawResult struct {
	Subject string
	CA      float64
	Exam    float64
}

type ReportRow struct {
	Subject string
	CA      float64
	Exam    float64
	Total   float64
	Grade   string
	Remark  string
}

type Report struct {
	Rows    []ReportRow
	Average float64
	Grade   string
	Remark  string
}

// BuildReport turns raw scores into a report card with totals, grades and an average.
func BuildReport(raw []RawResult) Report {
	var rep Report
	var sum float64
	for _, r := range raw {
		total := r.CA + r.Exam
		g, remark := Grade(total)
		rep.Rows = append(rep.Rows, ReportRow{r.Subject, r.CA, r.Exam, total, g, remark})
		sum += total
	}
	if len(rep.Rows) > 0 {
		rep.Average = math.Round(sum/float64(len(rep.Rows))*10) / 10
		rep.Grade, rep.Remark = Grade(rep.Average)
	}
	return rep
}
