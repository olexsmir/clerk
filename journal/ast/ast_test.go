package ast

import "testing"

func TestDateCompare(t *testing.T) {
	jan1 := Date{Year: 2024, Month: 1, Day: 1}
	jan2 := Date{Year: 2024, Month: 1, Day: 2}
	feb1 := Date{Year: 2024, Month: 2, Day: 1}
	nextYear := Date{Year: 2025, Month: 1, Day: 1}

	tests := []struct {
		a, b Date
		want int
	}{
		{jan1, jan1, 0},
		{jan2, jan1, 1},
		{jan1, jan2, -1},
		{feb1, jan2, 1},
		{jan2, feb1, -1},
		{nextYear, feb1, 1},
		{feb1, nextYear, -1},
	}
	for _, c := range tests {
		if got := c.a.Compare(c.b); got != c.want {
			t.Errorf("Compare(%v, %v) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
