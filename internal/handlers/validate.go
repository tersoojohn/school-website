package handlers

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	phoneRe   = regexp.MustCompile(`^[0-9+() \-]{7,20}$`)
	accountRe = regexp.MustCompile(`^[0-9]{10}$`) // Nigerian NUBAN account numbers are 10 digits
	codeRe    = regexp.MustCompile(`^[A-Z0-9]{2,12}$`)
)

// text trims s and checks its length in characters.
func text(s string, min, max int) (string, bool) {
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	return s, n >= min && n <= max
}

func optionalPhone(s string) (string, bool) {
	s = strings.TrimSpace(s)
	return s, s == "" || phoneRe.MatchString(s)
}

func optionalDate(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", true
	}
	t, err := time.Parse("2006-01-02", s)
	return s, err == nil && t.Before(time.Now())
}
