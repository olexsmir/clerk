package decimal

import (
	"fmt"
	"math/big"
	"strings"
)

type Decimal struct {
	scale int
	coeff *big.Int
}

func FromInt(v int64) Decimal {
	if v == 0 {
		return Decimal{}
	}
	return Decimal{coeff: big.NewInt(v)}
}

func FromString(s string) (Decimal, error) {
	original := s
	if s == "" {
		return Decimal{}, fmt.Errorf("can't convert %s to decimal", original)
	}

	sign := 1
	if s[0] == '+' || s[0] == '-' {
		if s[0] == '-' {
			sign = -1
		}
		s = s[1:]
	}

	if s == "" {
		return Decimal{}, fmt.Errorf("can't convert %s to decimal", original)
	}

	firstDot := strings.IndexByte(s, '.')
	lastDot := strings.LastIndexByte(s, '.')
	if firstDot != lastDot {
		return Decimal{}, fmt.Errorf("can't convert %s to decimal", original)
	}

	intPart := s
	fracPart := ""
	if firstDot >= 0 {
		intPart = s[:firstDot]
		fracPart = s[firstDot+1:]
	}

	if len(intPart)+len(fracPart) == 0 {
		return Decimal{}, fmt.Errorf("can't convert %s to decimal", original)
	}

	for i := 0; i < len(intPart); i++ {
		if intPart[i] < '0' || intPart[i] > '9' {
			return Decimal{}, fmt.Errorf("can't convert %s to decimal", original)
		}
	}
	for i := 0; i < len(fracPart); i++ {
		if fracPart[i] < '0' || fracPart[i] > '9' {
			return Decimal{}, fmt.Errorf("can't convert %s to decimal", original)
		}
	}

	coeffDigits := intPart + fracPart
	coeff := new(big.Int)
	if _, ok := coeff.SetString(coeffDigits, 10); !ok {
		return Decimal{}, fmt.Errorf("can't convert %s to decimal", original)
	}
	if sign < 0 && coeff.Sign() != 0 {
		coeff.Neg(coeff)
	}

	out := Decimal{coeff: coeff, scale: len(fracPart)}
	return out.normalized(), nil
}

func (d Decimal) String() string {
	if d.coeff == nil || d.coeff.Sign() == 0 {
		return "0"
	}

	abs := new(big.Int).Set(d.coeff)
	sign := ""
	if abs.Sign() < 0 {
		sign = "-"
		abs.Abs(abs)
	}

	digits := abs.String()
	if d.scale == 0 {
		return sign + digits
	}

	if len(digits) <= d.scale {
		digits = strings.Repeat("0", d.scale-len(digits)+1) + digits
	}
	split := len(digits) - d.scale
	return sign + digits[:split] + "." + digits[split:]
}

func (d Decimal) Neg() Decimal {
	if d.coeff == nil || d.coeff.Sign() == 0 {
		return Decimal{}
	}
	return Decimal{coeff: new(big.Int).Neg(d.coeff), scale: d.scale}
}

func (d Decimal) Sub(other Decimal) Decimal { return d.Add(other.Neg()) }
func (d Decimal) Add(other Decimal) Decimal {
	a, b, scale := align(d, other)
	sum := new(big.Int).Add(a, b)
	return Decimal{coeff: sum, scale: scale}.normalized()
}

func (d Decimal) Mul(other Decimal) Decimal {
	if d.IsZero() || other.IsZero() {
		return Decimal{}
	}
	product := new(big.Int).Mul(d.coeffOrZero(), other.coeffOrZero())
	return Decimal{coeff: product, scale: d.scale + other.scale}.normalized()
}

func (d Decimal) Cmp(other Decimal) int {
	a, b, _ := align(d, other)
	return a.Cmp(b)
}

func (d Decimal) Equal(other Decimal) bool {
	return d.Cmp(other) == 0
}

func (d Decimal) IsZero() bool {
	return d.coeff == nil || d.coeff.Sign() == 0
}

func align(a, b Decimal) (aCoeff *big.Int, bCoeff *big.Int, scale int) {
	scale = max(b.scale, a.scale)

	aCoeff = a.coeffOrZero()
	bCoeff = b.coeffOrZero()
	if delta := scale - a.scale; delta > 0 {
		aCoeff.Mul(aCoeff, pow10(delta))
	}
	if delta := scale - b.scale; delta > 0 {
		bCoeff.Mul(bCoeff, pow10(delta))
	}
	return aCoeff, bCoeff, scale
}

func (d Decimal) coeffOrZero() *big.Int {
	if d.coeff == nil {
		return new(big.Int)
	}
	return new(big.Int).Set(d.coeff)
}

func (d Decimal) normalized() Decimal {
	if d.coeff == nil || d.coeff.Sign() == 0 {
		return Decimal{}
	}
	if d.scale == 0 {
		return Decimal{coeff: new(big.Int).Set(d.coeff)}
	}

	sign := d.coeff.Sign()
	abs := new(big.Int).Abs(d.coeff)
	ten := big.NewInt(10)
	rem := new(big.Int)
	for d.scale > 0 {
		quotient, _ := new(big.Int).QuoRem(abs, ten, rem)
		if rem.Sign() != 0 {
			break
		}
		abs = quotient
		d.scale--
	}

	if sign < 0 {
		abs.Neg(abs)
	}
	return Decimal{coeff: abs, scale: d.scale}
}

func pow10(n int) *big.Int {
	if n <= 0 {
		return big.NewInt(1)
	}
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}
