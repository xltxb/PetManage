package money

import (
	"fmt"
	"strconv"
	"strings"
)

// ToYuan converts an amount in cents (bigint) to a display string in yuan.
// e.g., 26800 → "268.00", -100 → "-1.00"
func ToYuan(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	yuan := cents / 100
	frac := cents % 100
	return fmt.Sprintf("%s%d.%02d", sign, yuan, frac)
}

// ToCents converts a yuan string to cents (bigint).
// e.g., "268.00" → 26800
func ToCents(yuan string) (int64, error) {
	yuan = strings.TrimSpace(yuan)
	if yuan == "" {
		return 0, fmt.Errorf("empty amount string")
	}
	parts := strings.Split(yuan, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid amount format: %s", yuan)
	}
	intPart, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount: %s", yuan)
	}
	var fracPart int64
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) == 1 {
			frac += "0"
		}
		if len(frac) > 2 {
			return 0, fmt.Errorf("invalid fractional part: %s", yuan)
		}
		fracPart, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid amount: %s", yuan)
		}
	}
	negative := intPart < 0
	if negative {
		intPart = -intPart
	}
	cents := intPart*100 + fracPart
	if negative {
		cents = -cents
	}
	return cents, nil
}
