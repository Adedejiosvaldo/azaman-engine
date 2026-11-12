package parser

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	amountSanitizeRe = regexp.MustCompile(`[₦$€£,\s"']`)
	currencyCodeRe   = regexp.MustCompile(`(?i)NGN|USD|GBP|EUR`)
)

// Date layouts (no time). Day comes before month, as Nigerian banks write it; month-first is
// tried last for dollar-account exports ("01/15/2026").
var dateOnlyLayouts = []string{
	"02-Jan-2006", "2-Jan-2006", "02-Jan-06", "2-Jan-06", "02/Jan/2006", "02/Jan/06", "02Jan2006",
	"02/01/2006", "2/1/2006", "02/01/06", "2/1/06",
	"2006-01-02", "2006/01/02", "2006.01.02",
	"02-01-2006", "2-1-2006", "02-01-06",
	"02.01.2006", "02.01.06",
	"02 Jan 2006", "2 Jan 2006", "02 Jan 06", "2 Jan 06", "02 January 2006", "2 January 2006",
	"Jan 02 2006", "Jan 2 2006", "January 02 2006", "January 2 2006",
	"20060102",
	"01/02/2006", "1/2/2006",
}

var timeLayouts = []string{"15:04:05", "15:04", "3:04:05 PM", "3:04 PM", "3:04:05PM", "3:04PM", "15:04:05 PM"}

var (
	// "10:15", "10:15:22", "10:15:22.123", "3:04 PM", with an optional zone the layouts ignore
	timePartRe = regexp.MustCompile(`(?i)[ T](\d{1,2}:\d{2}(?::\d{2})?)(?:\.\d+)?\s?([AP]M)?(?:\s?(?:Z|WAT|GMT|UTC|[+-]\d{2}:?\d{2}))?$`)
	ordinalRe  = regexp.MustCompile(`(?i)\b(\d{1,2})(?:st|nd|rd|th)\b`)
	weekdayRe  = regexp.MustCompile(`(?i)^(?:mon|tue|tues|wed|thu|thur|thurs|fri|sat|sun)[a-z]*[, ]+`)
	monthFixRe = strings.NewReplacer("Sept ", "Sep ", "Sept-", "Sep-", "Sept/", "Sep/")
)

// ParseDate reads a date, with or without a time, in the formats banks print. The date and the
// time are parsed separately, so any date style combines with any time style. Years outside
// 1990 to next year are rejected: an eight-digit reference is not a date.
func ParseDate(dateStr string) (time.Time, bool) {
	s := strings.TrimSpace(dateStr)
	if len(s) < 6 || len(s) > 40 {
		return time.Time{}, false
	}
	s = strings.Join(strings.Fields(strings.ReplaceAll(s, ",", " ")), " ")
	s = weekdayRe.ReplaceAllString(s, "")
	s = ordinalRe.ReplaceAllString(s, "$1")

	clock := ""
	if m := timePartRe.FindStringSubmatchIndex(s); m != nil {
		clock = s[m[2]:m[3]]
		if m[4] >= 0 {
			clock += " " + strings.ToUpper(s[m[4]:m[5]])
		}
		s = strings.TrimSpace(s[:m[0]])
	}

	// Month names in any case: "02-JAN-2024", "02-jan-2024" -> "02-Jan-2024"
	s = monthFixRe.Replace(titleWords(s) + " ")
	s = strings.TrimSpace(s)

	for _, layout := range dateOnlyLayouts {
		day, err := time.Parse(layout, s)
		if err != nil {
			continue
		}
		if y := day.Year(); y < 1990 || y > time.Now().Year()+1 {
			continue
		}
		if clock == "" {
			return day, true
		}
		for _, tl := range timeLayouts {
			if t, err := time.Parse(tl, clock); err == nil {
				return day.Add(time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute + time.Duration(t.Second())*time.Second), true
			}
		}
		return day, true // unreadable time: keep the date
	}
	return time.Time{}, false
}

// titleWords upper-cases the first letter of each run of letters and lower-cases the rest.
func titleWords(s string) string {
	out := []rune(strings.ToLower(s))
	start := true
	for i, r := range out {
		letter := r >= 'a' && r <= 'z'
		if letter && start {
			out[i] = r - 'a' + 'A'
		}
		start = !letter
	}
	return string(out)
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
