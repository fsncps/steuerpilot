package util

import (
	"fmt"
	"math"
	"strings"
)

// FormatCHF formats a float64 as a Swiss franc amount with apostrophe thousand separator.
// Example: 1234567.89 → "CHF 1'234'567.89"
func FormatCHF(amount float64) string {
	return "CHF " + formatNumber(amount, 2)
}

// FormatCHFRound formats without decimal places, rounding to nearest franc.
// Example: 1234.6 → "CHF 1'235"
func FormatCHFRound(amount float64) string {
	return "CHF " + formatNumber(math.Round(amount), 0)
}

func formatNumber(amount float64, decimals int) string {
	format := fmt.Sprintf("%%.%df", decimals)
	s := fmt.Sprintf(format, amount)

	parts := strings.SplitN(s, ".", 2)
	intPart := parts[0]

	// Insert apostrophes as thousand separators
	var grouped strings.Builder
	for i, ch := range intPart {
		pos := len(intPart) - i
		if i > 0 && pos%3 == 0 {
			grouped.WriteRune('\'') // apostrophe
		}
		grouped.WriteRune(ch)
	}

	if decimals > 0 && len(parts) == 2 {
		return grouped.String() + "." + parts[1]
	}
	return grouped.String()
}
