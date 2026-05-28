package decimal

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

type Decimal struct {
	value *big.Rat
}

var (
	Zero = MustParse("0")
	One  = MustParse("1")
)

func Parse(input string) (Decimal, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return Decimal{}, errors.New("empty decimal")
	}
	r := new(big.Rat)
	if _, ok := r.SetString(s); !ok {
		return Decimal{}, fmt.Errorf("invalid decimal %q", input)
	}
	return Decimal{value: r}, nil
}

func MustParse(input string) Decimal {
	d, err := Parse(input)
	if err != nil {
		panic(err)
	}
	return d
}

func NewFromInt(v int64) Decimal {
	return Decimal{value: new(big.Rat).SetInt64(v)}
}

func (d Decimal) Add(other Decimal) Decimal {
	return Decimal{value: new(big.Rat).Add(d.rat(), other.rat())}
}

func (d Decimal) Sub(other Decimal) Decimal {
	return Decimal{value: new(big.Rat).Sub(d.rat(), other.rat())}
}

func (d Decimal) Mul(other Decimal) Decimal {
	return Decimal{value: new(big.Rat).Mul(d.rat(), other.rat())}
}

func (d Decimal) Neg() Decimal {
	return Decimal{value: new(big.Rat).Neg(d.rat())}
}

func (d Decimal) Min(other Decimal) Decimal {
	if d.Cmp(other) <= 0 {
		return d
	}
	return other
}

func (d Decimal) Round(scale int) Decimal {
	if scale < 0 {
		scale = 0
	}

	mult := pow10(scale)
	scaled := new(big.Rat).Mul(d.rat(), new(big.Rat).SetInt(mult))
	num := new(big.Int).Set(scaled.Num())
	den := new(big.Int).Set(scaled.Denom())
	negative := num.Sign() < 0
	if negative {
		num.Abs(num)
	}

	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(num, den, remainder)
	doubleRemainder := new(big.Int).Mul(remainder, big.NewInt(2))
	if doubleRemainder.Cmp(den) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if negative {
		quotient.Neg(quotient)
	}
	return Decimal{value: new(big.Rat).SetFrac(quotient, mult)}
}

func (d Decimal) Cmp(other Decimal) int {
	return d.rat().Cmp(other.rat())
}

func (d Decimal) Sign() int {
	return d.rat().Sign()
}

func (d Decimal) IsZero() bool {
	return d.Sign() == 0
}

func (d Decimal) String() string {
	if d.value == nil {
		return "0"
	}
	s := d.value.FloatString(10)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}

func (d Decimal) Fixed(scale int) string {
	return d.rat().FloatString(scale)
}

func (d Decimal) MarshalJSON() ([]byte, error) {
	return []byte(d.String()), nil
}

func (d *Decimal) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "null" || raw == "" {
		*d = Zero
		return nil
	}
	if strings.HasPrefix(raw, "\"") {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		raw = s
	}
	parsed, err := Parse(raw)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d Decimal) Value() (driver.Value, error) {
	return d.String(), nil
}

func (d *Decimal) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*d = Zero
		return nil
	case string:
		parsed, err := Parse(v)
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	case []byte:
		parsed, err := Parse(string(v))
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	case int64:
		*d = NewFromInt(v)
		return nil
	case float64:
		parsed, err := Parse(fmt.Sprintf("%.10f", v))
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	default:
		return fmt.Errorf("cannot scan %T into Decimal", src)
	}
}

func (d Decimal) rat() *big.Rat {
	if d.value == nil {
		return new(big.Rat)
	}
	return new(big.Rat).Set(d.value)
}

func pow10(scale int) *big.Int {
	out := big.NewInt(1)
	ten := big.NewInt(10)
	for i := 0; i < scale; i++ {
		out.Mul(out, ten)
	}
	return out
}
