package decimal

import (
	"fmt"
	"math/big"
	"strconv"
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
		return Decimal{}, badDecimal(original)
	}

	neg := false
	if s[0] == '+' || s[0] == '-' {
		neg = s[0] == '-'
		s = s[1:]
	}

	exp := 0
	if e := strings.IndexAny(s, "eE"); e >= 0 {
		n, err := strconv.Atoi(s[e+1:])
		if err != nil {
			return Decimal{}, badDecimal(original)
		}
		// bounded so pow10 can't be asked to materialize an astronomically
		// large number (e.g. 1E1000000000000)
		const maxExp = 10000
		if n > maxExp || n < -maxExp {
			return Decimal{}, badDecimal(original)
		}
		exp = n
		s = s[:e]
	}

	intPart, fracPart := s, ""
	if before, after, ok := strings.Cut(s, "."); ok {
		if strings.IndexByte(after, '.') >= 0 {
			return Decimal{}, badDecimal(original)
		}
		intPart, fracPart = before, after
	}

	digits := intPart + fracPart
	if neg {
		digits = "-" + digits
	}
	coeff, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return Decimal{}, badDecimal(original)
	}

	scale := len(fracPart) - exp
	if scale < 0 {
		coeff.Mul(coeff, pow10(-scale))
		scale = 0
	}
	return Decimal{coeff: coeff, scale: scale}.normalized(), nil
}

func badDecimal(s string) error { return fmt.Errorf("can't convert %s to decimal", s) }

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

// StringFixed returns a string representation with exactly places digits
// after the decimal point. Pads with zeros or truncates as needed.
// decSep and thousandsSep control formatting; zero values mean no custom separator.
func (d Decimal) StringFixed(places int, decSep, thousandsSep byte) string {
	var sb strings.Builder
	sb.Grow(32)
	d.WriteFixed(&sb, places, decSep, thousandsSep)
	return sb.String()
}

// WriteFixed writes a string representation with exactly places digits
// after the decimal point directly into sb. Pads with zeros or truncates
// as needed. decSep and thousandsSep control formatting; zero values mean
// no custom separator.
func (d Decimal) WriteFixed(sb *strings.Builder, places int, decSep, thousandsSep byte) {
	if d.IsZero() {
		sb.WriteByte('0')
		if places > 0 {
			if decSep != 0 {
				sb.WriteByte(decSep)
			} else {
				sb.WriteByte('.')
			}
			for range places {
				sb.WriteByte('0')
			}
		}
		return
	}

	var digitBuf [128]byte
	digits := d.coeff.Append(digitBuf[:0], 10)

	sign := false
	if len(digits) > 0 && digits[0] == '-' {
		sign = true
		digits = digits[1:]
	}

	intLen := len(digits) - d.scale

	if sign {
		sb.WriteByte('-')
	}

	// Integer part
	if intLen <= 0 {
		sb.WriteByte('0')
	} else {
		for i := range intLen {
			if thousandsSep != 0 && i > 0 && (intLen-i)%3 == 0 {
				sb.WriteByte(thousandsSep)
			}
			sb.WriteByte(digits[i])
		}
	}

	// Fractional part
	if places > 0 {
		if decSep != 0 {
			sb.WriteByte(decSep)
		} else {
			sb.WriteByte('.')
		}

		written := 0
		if intLen > 0 {
			n := min(d.scale, places)
			for i := range n {
				sb.WriteByte(digits[intLen+i])
				written++
			}
		} else {
			leadingZeros := -intLen
			for ; written < leadingZeros && written < places; written++ {
				sb.WriteByte('0')
			}
			for i := 0; i < len(digits) && written < places; i++ {
				sb.WriteByte(digits[i])
				written++
			}
		}
		for ; written < places; written++ {
			sb.WriteByte('0')
		}
	}
}

func (d Decimal) Abs() Decimal {
	if d.coeff == nil || d.coeff.Sign() == 0 {
		return Decimal{}
	}
	if d.coeff.Sign() > 0 {
		return Decimal{coeff: new(big.Int).Set(d.coeff), scale: d.scale}
	}
	return Decimal{coeff: new(big.Int).Neg(d.coeff), scale: d.scale}
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

func (d Decimal) Div(other Decimal) Decimal {
	if other.IsZero() {
		panic("decimal: division by zero")
	}
	if d.IsZero() {
		return Decimal{}
	}

	scale := max(d.scale, other.scale) + 10

	dCoeff := d.coeffOrZero()
	oCoeff := other.coeffOrZero()

	shift := scale + other.scale - d.scale
	if shift > 0 {
		dCoeff = new(big.Int).Mul(dCoeff, pow10(shift))
	} else if shift < 0 {
		oCoeff = new(big.Int).Mul(oCoeff, pow10(-shift))
	}

	quo := new(big.Int).Quo(dCoeff, oCoeff)
	return Decimal{coeff: quo, scale: scale}.normalized()
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

func pow10(n int) *big.Int {
	if n <= 0 {
		return big.NewInt(1)
	}
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}
