package ast

import "testing"

func TestDateCompare(t *testing.T) {
	jan1 := Date{Year: 2024, Month: 1, Day: 1}
	jan2 := Date{Year: 2024, Month: 1, Day: 2}
	feb1 := Date{Year: 2024, Month: 2, Day: 1}
	nextYear := Date{Year: 2025, Month: 1, Day: 1}

	tests := map[[2]Date]int{
		{jan1, jan1}:     0,
		{jan2, jan1}:     1,
		{jan1, jan2}:     -1,
		{feb1, jan2}:     1,
		{jan2, feb1}:     -1,
		{nextYear, feb1}: 1,
		{feb1, nextYear}: -1,
	}
	for dates, want := range tests {
		if got := dates[0].Compare(dates[1]); got != want {
			t.Errorf("Compare(%v, %v) = %d, want %d", dates[0], dates[1], got, want)
		}
	}
}
