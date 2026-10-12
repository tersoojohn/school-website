// Package services holds business rules that do not touch HTTP or SQL.
package services

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	MaxCA   = 40.0
	MaxExam = 60.0
)

// Grade maps a total out of 100 to a letter and a short remark.
func Grade(total float64) (letter, remark string) {
	switch {
	case total >= 70:
		return "A", "Excellent"
	case total >= 60:
		return "B", "Very good"
	case total >= 50:
		return "C", "Credit"
	case total >= 45:
		return "D", "Pass"
	case total >= 40:
		return "E", "Weak pass"
	}
	return "F", "Fail"
}

// ParseScore reads one score field. An empty field means "not entered yet" and returns nil.
func ParseScore(raw string, max float64) (*float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return nil, errors.New("must be a number")
	}
	if v < 0 || v > max {
		return nil, fmt.Errorf("must be between 0 and %g", max)
	}
	v = math.Round(v*10) / 10
	return &v, nil
}
