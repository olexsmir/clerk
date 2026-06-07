package decimal

import "testing"

func TestNewFromString(t *testing.T) {
	tests := []struct{ in, want string }{
		{"10.00", "10"},
		{"150.60", "150.6"},
		{".33", "0.33"},
		{"1.", "1"},
		{"0001.2300", "1.23"},
		{"0", "0"},
		{"0.00", "0"},
		{"-0.00", "0"},
		{"-20", "-20"},
		{"-.75", "-0.75"},
	}
	for _, tt := range tests {
		got, err := FromString(tt.in)
		if err != nil {
			t.Fatalf("NewFromString(%q) unexpected error: %v", tt.in, err)
		}
		if got.String() != tt.want {
			t.Fatalf("NewFromString(%q).String() = %q, want %q", tt.in, got.String(), tt.want)
		}
	}
}

func TestNewFromStringInvalid(t *testing.T) {
	tests := []string{
		"", ".", "+", "-",
		"1_000.00", "1,000.00",
		"1..0",
		"a1", "1a",
	}
	for _, in := range tests {
		if _, err := FromString(in); err == nil {
			t.Fatalf("NewFromString(%q) expected error", in)
		}
	}
}

func TestStringFixed(t *testing.T) {
	tests := []struct {
		in           string
		places       int
		decSep       byte
		thousandsSep byte
		want         string
	}{
		{"0", 0, 0, 0, "0"},
		{"0", 1, 0, 0, "0.0"},
		{"0", 2, 0, 0, "0.00"},
		{"0", 3, 0, 0, "0.000"},
		{"-0", 2, 0, 0, "0.00"},
		{"10", 2, 0, 0, "10.00"},
		{"10", 0, 0, 0, "10"},
		{"1.2", 2, 0, 0, "1.20"},
		{"1.2", 1, 0, 0, "1.2"},
		{"1.2", 3, 0, 0, "1.200"},
		{"1.234", 2, 0, 0, "1.23"},
		{"1.235", 2, 0, 0, "1.23"},
		{"-1.2", 2, 0, 0, "-1.20"},
		{"-1.234", 2, 0, 0, "-1.23"},
		{"0.001", 2, 0, 0, "0.00"},
		{"0.001", 4, 0, 0, "0.0010"},
		{"123.456789", 3, 0, 0, "123.456"},
		{"-123.456789", 3, 0, 0, "-123.456"},
		{"1.23", 2, ',', 0, "1,23"},
		{"0", 2, ',', 0, "0,00"},
		{"-1.5", 2, ',', 0, "-1,50"},
		{"1234.56", 2, 0, ',', "1,234.56"},
		{"1234567.89", 2, 0, ',', "1,234,567.89"},
		{"123.45", 2, 0, ',', "123.45"},
		{"-1234.56", 2, 0, ',', "-1,234.56"},
		{"1234567.89", 2, ',', '.', "1.234.567,89"},
		{"-1234567.89", 2, ',', '.', "-1.234.567,89"},
		{"0", 2, ',', '.', "0,00"},
	}
	for _, tt := range tests {
		d, err := FromString(tt.in)
		if err != nil {
			t.Fatalf("FromString(%q) unexpected error: %v", tt.in, err)
		}
		if got := d.StringFixed(tt.places, tt.decSep, tt.thousandsSep); got != tt.want {
			t.Fatalf("FromString(%q).StringFixed(%d, %q, %q) = %q, want %q",
				tt.in, tt.places, tt.decSep, tt.thousandsSep, got, tt.want)
		}
	}
}

func TestArithmetic(t *testing.T) {
	a, _ := FromString("1.20")
	b, _ := FromString(".30")

	if got := a.Add(b).String(); got != "1.5" {
		t.Fatalf("a+b = %q, want %q", got, "1.5")
	}
	if got := a.Sub(b).String(); got != "0.9" {
		t.Fatalf("a-b = %q, want %q", got, "0.9")
	}
	if got := a.Mul(b).String(); got != "0.36" {
		t.Fatalf("a*b = %q, want %q", got, "0.36")
	}
}

func TestCmpAndNeg(t *testing.T) {
	a, _ := FromString("1.0")
	b, _ := FromString("1")
	c, _ := FromString("1.01")

	if a.Cmp(b) != 0 {
		t.Fatalf("expected %q and %q to be equal", a.String(), b.String())
	}
	if b.Cmp(c) >= 0 {
		t.Fatalf("expected %q to be less than %q", b.String(), c.String())
	}
	if got := b.Neg().String(); got != "-1" {
		t.Fatalf("Neg(1) = %q, want %q", got, "-1")
	}
	if got := (Decimal{}).Neg().String(); got != "0" {
		t.Fatalf("Neg(0) = %q, want %q", got, "0")
	}
}
