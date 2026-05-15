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
		"",
		".",
		"+",
		"-",
		"1_000.00",
		"1,000.00",
		"1..0",
		"a1",
		"1a",
	}
	for _, in := range tests {
		if _, err := FromString(in); err == nil {
			t.Fatalf("NewFromString(%q) expected error", in)
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
