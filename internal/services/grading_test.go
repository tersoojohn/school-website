package services

import "testing"

func TestGrade(t *testing.T) {
	cases := map[float64]string{100: "A", 70: "A", 69.9: "B", 60: "B", 50: "C", 45: "D", 40: "E", 39.9: "F", 0: "F"}
	for total, want := range cases {
		if got, _ := Grade(total); got != want {
			t.Errorf("Grade(%v) = %s, want %s", total, got, want)
		}
	}
}

func TestParseScore(t *testing.T) {
	if v, err := ParseScore("", MaxCA); v != nil || err != nil {
		t.Error("empty should mean not entered")
	}
	if v, err := ParseScore(" 35.46 ", MaxCA); err != nil || *v != 35.5 {
		t.Errorf("got %v %v", v, err)
	}
	for _, bad := range []string{"41", "-1", "abc", "NaN", "Inf"} {
		if _, err := ParseScore(bad, MaxCA); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestBuildReport(t *testing.T) {
	rep := BuildReport([]RawResult{{"Maths", 30, 50}, {"English", 20, 30}})
	if rep.Average != 65 || rep.Grade != "B" {
		t.Errorf("unexpected report: %+v", rep)
	}
}

func TestSummarizeAndDistribution(t *testing.T) {
	totals := []float64{85, 72, 55, 38, 40}
	s := Summarize(totals)
	if s.Count != 5 || s.Mean != 58 || s.Median != 55 || s.Min != 38 || s.Max != 85 || s.PassRate != 80 {
		t.Errorf("unexpected summary: %+v", s)
	}
	d := Distribution(totals)
	want := map[string]int{"A": 2, "B": 0, "C": 1, "D": 0, "E": 1, "F": 1}
	for _, lc := range d {
		if want[lc.Label] != lc.Value {
			t.Errorf("grade %s: got %d want %d", lc.Label, lc.Value, want[lc.Label])
		}
	}
	if Summarize(nil).Count != 0 {
		t.Error("empty summary should be zero")
	}
}
