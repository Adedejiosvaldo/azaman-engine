package parser

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// Known date formats commonly used in Nigerian and international statements
	dateFormats = []string{
		"02-Jan-2006",
		"02-Jan-06",
		"02/Jan/2006",
		"02-JAN-2006",
		"2-Jan-2006",
		"02/01/2006",
		"2/1/2006",
		"02/01/06",
		"2006-01-02",
		"2006/01/02",
		"02-01-2006",
		"02 Jan 2006",
		"Jan 02, 2006",
		"Jan 2, 2006",
		"January 02, 2006",
		"January 2, 2006",
		"02/01/2006 15:04:05",
		"02/01/2006 15:04",
		"02/01/2006 03:04 PM",
		"02/01/2006 3:04 PM",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"02 Jan 2006 15:04:05",
		"02 Jan 2006 15:04",
		"2 Jan 2006 15:04:05",
		"2 Jan 2006 15:04",
		"02-Jan-2006 15:04:05",
		"02-Jan-2006 15:04",
		// Additional layouts seen on Zenith, First Bank, UBA, PalmPay and Moniepoint exports
		"02-Jan-06 15:04:05",
		"02-Jan-06 15:04",
		"02 Jan 2006, 15:04",
		"02 Jan 2006, 15:04:05",
		"02 Jan 2006 03:04 PM",
		"02 Jan 2006 3:04 PM",
		"Jan 2, 2006 15:04",
		"Jan 2, 2006 3:04 PM",
		"Jan 02 2006",
		"02 January 2006",
		"2 January 2006",
		"Mon, 02 Jan 2006 15:04:05",
		"Monday, January 2, 2006",
		"02.01.2006",
		"02.01.2006 15:04",
		"02-01-2006 15:04:05",
		"02-01-2006 15:04",
		"02/01/2006 15:04:05 PM",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05.000",
		"20060102",
	}

	amountSanitizeRe = regexp.MustCompile(`[₦$€£,\s"']`)
	currencyCodeRe   = regexp.MustCompile(`(?i)NGN|USD|GBP|EUR`)
)

// ParseDate attempts to parse a string using known Nigerian and international date formats.
func ParseDate(dateStr string) (time.Time, bool) {
	trimmed := strings.TrimSpace(dateStr)
	if trimmed == "" {
		return time.Time{}, false
	}

	// Try direct uppercase matching (for 02-JAN-2024)
	for _, layout := range dateFormats {
		if t, err := time.Parse(layout, trimmed); err == nil {
			return t, true
		}
		// Try case-insensitive matching by title casing month (e.g. 15-jan-2024 -> 15-Jan-2024)
		if t, err := time.Parse(layout, strings.Title(strings.ToLower(trimmed))); err == nil {
			return t, true
		}
	}

	return time.Time{}, false
}

// ParseAmount parses an amount string into a float64.
// Handles negative representations like "-500", "(500.00)", "500.00 DR", and trims currency symbols.
func ParseAmount(amountStr string) float64 {
	trimmed := strings.TrimSpace(amountStr)
	if trimmed == "" || trimmed == "-" || trimmed == "--" {
		return 0.0
	}

	isNegative := false

	// Check parenthesis format: (1,500.00)
	if strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")") {
		isNegative = true
		trimmed = trimmed[1 : len(trimmed)-1]
	}

	// Check suffix DR / CR
	upper := strings.ToUpper(trimmed)
	if strings.HasSuffix(upper, "DR") {
		isNegative = true
		trimmed = strings.TrimSpace(trimmed[:len(trimmed)-2])
	} else if strings.HasSuffix(upper, "CR") {
		trimmed = strings.TrimSpace(trimmed[:len(trimmed)-2])
	}

	// Remove currency symbols, commas, quotes
	sanitized := amountSanitizeRe.ReplaceAllString(trimmed, "")
	sanitized = currencyCodeRe.ReplaceAllString(sanitized, "")
	sanitized = strings.TrimSpace(sanitized)

	// Trailing minus used by some core-banking exports: "1,200.00-"
	if strings.HasSuffix(sanitized, "-") {
		isNegative = true
		sanitized = strings.TrimSuffix(sanitized, "-")
	}
	// Naira written as a plain "N" prefix: "N1,200.00", "-N500"
	sanitized = strings.TrimPrefix(sanitized, "N")
	if strings.HasPrefix(sanitized, "-N") {
		sanitized = "-" + sanitized[2:]
	}

	if strings.HasPrefix(sanitized, "-") {
		isNegative = true
		sanitized = strings.TrimPrefix(sanitized, "-")
	} else if strings.HasPrefix(sanitized, "+") {
		sanitized = strings.TrimPrefix(sanitized, "+")
	}

	val, err := strconv.ParseFloat(sanitized, 64)
	if err != nil {
		return 0.0
	}

	if isNegative {
		return -val
	}
	return val
}

// DetectCurrency checks whether the document or line specifies USD, GBP, or defaults to NGN.
func DetectCurrency(sample string) string {
	upper := strings.ToUpper(sample)

	// Check explicit currency header or symbols
	if strings.Contains(upper, "CURRENCY: NGN") || strings.Contains(upper, "CURRENCY: NAIRA") || strings.Contains(upper, "₦") {
		return "NGN"
	}
	if strings.Contains(upper, "CURRENCY: USD") || strings.Contains(upper, "USD WALLET") || strings.Contains(upper, "CURRENCY: $") {
		return "USD"
	}

	// Prioritize NGN for Nigerian domestic accounts
	if strings.Contains(upper, "NGN") {
		return "NGN"
	}
	if strings.Contains(upper, "USD") || strings.Contains(upper, "$") {
		return "USD"
	}
	if strings.Contains(upper, "GBP") || strings.Contains(upper, "£") {
		return "GBP"
	}
	if strings.Contains(upper, "EUR") || strings.Contains(upper, "€") {
		return "EUR"
	}
	return "NGN"
}
