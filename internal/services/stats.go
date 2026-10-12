package services

import (
	"math"
	"sort"
)

// PassMark is the total (out of 100) at or above which a subject counts as passed.
const PassMark = 40.0

// GradeLetters in display order.
var GradeLetters = []string{"A", "B", "C", "D", "E", "F"}

type Summary struct {
	Count    int
	Mean     float64
	Median   float64
	Min      float64
	Max      float64
	PassRate float64 // 0 to 100
}

// Summarize describes a list of totals. An empty list returns the zero Summary.
func Summarize(totals []float64) Summary {
	n := len(totals)
	if n == 0 {
		return Summary{}
	}
	s := append([]float64(nil), totals...)
	sort.Float64s(s)
	var sum float64
	passed := 0
	for _, v := range s {
		sum += v
		if v >= PassMark {
			passed++
		}
	}
	median := s[n/2]
	if n%2 == 0 {
		median = (s[n/2-1] + s[n/2]) / 2
	}
	return Summary{
		Count: n, Mean: Round1(sum / float64(n)), Median: Round1(median),
		Min: s[0], Max: s[n-1], PassRate: Round1(float64(passed) * 100 / float64(n)),
	}
}

// Distribution counts totals per grade letter, in A to F order.
func Distribution(totals []float64) []LabelCount {
	counts := map[string]int{}
	for _, v := range totals {
		g, _ := Grade(v)
		counts[g]++
	}
	out := make([]LabelCount, 0, len(GradeLetters))
	for _, g := range GradeLetters {
		out = append(out, LabelCount{Label: g, Value: counts[g]})
	}
	return out
}

type LabelCount struct {
	Label string
	Value int
}

func Round1(v float64) float64 { return math.Round(v*10) / 10 }
