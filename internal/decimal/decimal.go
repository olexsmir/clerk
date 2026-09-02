package decimal

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

type Decimal struct {
	v     int64
	scale int
	big   *big.Int
}

func FromInt(v int64) Decimal {
	if v == 0 {
		return Decimal{}
	}
	return Decimal{v: v}
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

	scale := len(fracPart) - exp
	if scale >= 0 {
		if v, ok := parseDigits(intPart + fracPart); ok {
			if neg {
				v = -v
			}
			return Decimal{v: v, scale: scale}.fastNormalized(), nil
		}
	}

	digits := intPart + fracPart
	if neg {
		digits = "-" + digits
	}
	coeff, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return Decimal{}, badDecimal(original)
	}
	if scale < 0 {
		coeff.Mul(coeff, pow10(-scale))
		scale = 0
	}
	return Decimal{big: coeff, scale: scale}.normalized(), nil
}

func badDecimal(s string) error { return fmt.Errorf("can't convert %s to decimal", s) }

func parseDigits(s string) (int64, bool) {
	var v int64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		d := int64(c - '0')
		if v > (math.MaxInt64-d)/10 {
			return 0, false
		}
		v = v*10 + d
	}
	return v, len(s) > 0
}

func (d Decimal) fastNormalized() Decimal {
	v, scale := d.v, d.scale
	for v != 0 && scale > 0 && v%10 == 0 {
		v /= 10
		scale--
	}
	if v == 0 {
		return Decimal{}
	}
	return Decimal{v: v, scale: scale}
}

func (d Decimal) String() string {
	if d.big != nil {
		return bigString(d.big, d.scale)
	}
	if d.v == 0 {
		return "0"
	}
	digits := strconv.FormatInt(d.v, 10)
	sign := ""
	if digits[0] == '-' {
		sign, digits = "-", digits[1:]
	}
	if d.scale == 0 {
		return sign + digits
	}
	if len(digits) <= d.scale {
		digits = strings.Repeat("0", d.scale-len(digits)+1) + digits
	}
	split := len(digits) - d.scale
	return sign + digits[:split] + "." + digits[split:]
}

func bigString(coeff *big.Int, scale int) string {
	if coeff.Sign() == 0 {
		return "0"
	}
	abs := new(big.Int).Set(coeff)
	sign := ""
	if abs.Sign() < 0 {
		sign = "-"
		abs.Abs(abs)
	}
	digits := abs.String()
	if scale == 0 {
		return sign + digits
	}
	if len(digits) <= scale {
		digits = strings.Repeat("0", scale-len(digits)+1) + digits
	}
	split := len(digits) - scale
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
	var digits []byte
	if d.big != nil {
		digits = d.big.Append(digitBuf[:0], 10)
	} else {
		digits = strconv.AppendInt(digitBuf[:0], d.v, 10)
	}

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
	if d.big != nil {
		if d.big.Sign() == 0 {
			return Decimal{}
		}
		if d.big.Sign() > 0 {
			return Decimal{big: new(big.Int).Set(d.big), scale: d.scale}
		}
		return Decimal{big: new(big.Int).Neg(d.big), scale: d.scale}
	}
	if d.v == 0 {
		return Decimal{}
	}
	if d.v < 0 {
		return Decimal{v: -d.v, scale: d.scale}
	}
	return Decimal{v: d.v, scale: d.scale}
}

func (d Decimal) Neg() Decimal {
	if d.big != nil {
		if d.big.Sign() == 0 {
			return Decimal{}
		}
		return Decimal{big: new(big.Int).Neg(d.big), scale: d.scale}
	}
	if d.v == 0 {
		return Decimal{}
	}
	return Decimal{v: -d.v, scale: d.scale}
}

func (d Decimal) Sub(other Decimal) Decimal { return d.Add(other.Neg()) }
func (d Decimal) Add(other Decimal) Decimal {
	if d.big == nil && other.big == nil {
		if r, ok := fastAdd(d, other); ok {
			return r
		}
	}
	a, b, scale := align(d, other)
	sum := new(big.Int).Add(a, b)
	return Decimal{big: sum, scale: scale}.normalized()
}

func fastAdd(a, b Decimal) (Decimal, bool) {
	scale := max(a.scale, b.scale)
	av, ok1 := scaleBy(a.v, scale-a.scale)
	bv, ok2 := scaleBy(b.v, scale-b.scale)
	if !ok1 || !ok2 {
		return Decimal{}, false
	}
	sum := av + bv
	if (av > 0 && bv > 0 && sum < 0) || (av < 0 && bv < 0 && sum >= 0) {
		return Decimal{}, false
	}
	return Decimal{v: sum, scale: scale}.fastNormalized(), true
}

func (d Decimal) Mul(other Decimal) Decimal {
	if d.IsZero() || other.IsZero() {
		return Decimal{}
	}
	product := new(big.Int).Mul(d.coeffBig(), other.coeffBig())
	return Decimal{big: product, scale: d.scale + other.scale}.normalized()
}

func (d Decimal) Div(other Decimal) Decimal {
	if other.IsZero() {
		panic("decimal: division by zero")
	}
	if d.IsZero() {
		return Decimal{}
	}

	scale := max(d.scale, other.scale) + 10

	dCoeff := d.coeffBig()
	oCoeff := other.coeffBig()

	shift := scale + other.scale - d.scale
	if shift > 0 {
		dCoeff = new(big.Int).Mul(dCoeff, pow10(shift))
	} else if shift < 0 {
		oCoeff = new(big.Int).Mul(oCoeff, pow10(-shift))
	}

	quo := new(big.Int).Quo(dCoeff, oCoeff)
	return Decimal{big: quo, scale: scale}.normalized()
}

func (d Decimal) Cmp(other Decimal) int {
	if d.big == nil && other.big == nil {
		if r, ok := fastCmp(d, other); ok {
			return r
		}
	}
	a, b, _ := align(d, other)
	return a.Cmp(b)
}

func fastCmp(a, b Decimal) (int, bool) {
	scale := max(a.scale, b.scale)
	av, ok1 := scaleBy(a.v, scale-a.scale)
	bv, ok2 := scaleBy(b.v, scale-b.scale)
	if !ok1 || !ok2 {
		return 0, false
	}
	switch {
	case av < bv:
		return -1, true
	case av > bv:
		return 1, true
	}
	return 0, true
}

func (d Decimal) Equal(other Decimal) bool {
	return d.Cmp(other) == 0
}

func (d Decimal) IsZero() bool {
	if d.big != nil {
		return d.big.Sign() == 0
	}
	return d.v == 0
}

func (d Decimal) coeffBig() *big.Int {
	if d.big != nil {
		return new(big.Int).Set(d.big)
	}
	return big.NewInt(d.v)
}

// normalized strips trailing zero digits from a big coefficient and demotes
// to the fast lane when the result fits in an int64.
func (d Decimal) normalized() Decimal {
	if d.big == nil || d.big.Sign() == 0 {
		return Decimal{}
	}

	sign := d.big.Sign()
	abs := new(big.Int).Abs(d.big)
	ten := big.NewInt(10)
	rem := new(big.Int)
	scale := d.scale
	for scale > 0 {
		quotient, _ := new(big.Int).QuoRem(abs, ten, rem)
		if rem.Sign() != 0 {
			break
		}
		abs = quotient
		scale--
	}

	if sign < 0 {
		abs.Neg(abs)
	}
	if abs.IsInt64() {
		return Decimal{v: abs.Int64(), scale: scale}
	}
	return Decimal{big: abs, scale: scale}
}

func align(a, b Decimal) (aCoeff *big.Int, bCoeff *big.Int, scale int) {
	scale = max(b.scale, a.scale)

	aCoeff = a.coeffBig()
	bCoeff = b.coeffBig()
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

// pow10i64 holds 10^n for n up to 18 (10^18 fits in an int64).
var pow10i64 = [...]int64{
	1, 10, 100, 1_000, 10_000, 100_000, 1_000_000, 10_000_000, 100_000_000,
	1_000_000_000, 10_000_000_000, 100_000_000_000, 1_000_000_000_000,
	10_000_000_000_000, 100_000_000_000_000, 1_000_000_000_000_000,
	10_000_000_000_000_000, 100_000_000_000_000_000, 1_000_000_000_000_000_000,
}

// scaleBy returns v·10^n; ok is false when the product overflows int64.
func scaleBy(v int64, n int) (int64, bool) {
	if n <= 0 {
		return v, true
	}
	if n >= len(pow10i64) {
		return 0, false
	}
	p := pow10i64[n]
	r := v * p
	return r, v == 0 || r/v == p
}
