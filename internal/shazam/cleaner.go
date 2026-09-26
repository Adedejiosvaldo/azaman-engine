package shazam

import (
	"regexp"
	"strings"
)

var (
	// Regex patterns to strip clearinghouse, session IDs, terminal hashes, and transaction routing noise.
	nipCrDrRe       = regexp.MustCompile(`(?i)\b(NIP/(?:CR|DR|TRF|OUTWARD|INWARD)|TRF(?:/FRM|/TO)?|NIP\s+(?:TRANSFER\s+TO|TRF\s+FROM))\b`)
	sessionIDRe     = regexp.MustCompile(`\b\d{16,30}\b`)                     // Long NIBSS session IDs
	rrnRe           = regexp.MustCompile(`(?i)\b(?:RRN|STAN|REF|TXNID|ID)[:\s]*[A-Z0-9_-]+\b`)
	posTerminalRe   = regexp.MustCompile(`(?i)\b(?:UBAS_\d+|POS\s+(?:W/D|DEBIT|PURCHASE|CASHOUT)|POS/\w+|TERM(?:INAL)?\s*ID[:\s]*\w+)\b`)
	dateTimeStampRe = regexp.MustCompile(`\b\d{4}[-/]\d{2}[-/]\d{2}(?:\s+\d{2}:\d{2}(?::\d{2})?)?\b|\b\d{2}[-/]\d{2}[-/]\d{4}\b|\b\d{2}:\d{2}(?::\d{2})?\b`)
	webPayRe        = regexp.MustCompile(`(?i)\b(?:WEB\s+(?:PURCHASE|PAYMENT)|PAYSTACK\s*\*|FLUTTERWAVE\s*\*|INTERSWITCH\s*\*|CORALPAY\s*\*)\b`)
	bankPrefixRe    = regexp.MustCompile(`(?i)\b(?:GTB|ACCESS|KUDA|ZENITH|FBN|UBA|STANBIC|MONIEPOINT|OPAY|PALMPAY|WEMA|PROVIDUS|FIDELITY|ECOBANK|FCMB|UNION|KEYSTONE|STERLING|JAIZ|TAJ)\b`)
	phoneRe         = regexp.MustCompile(`\b(?:\+?234|0)[789][01]\d{8}\b`)
	accountNumRe    = regexp.MustCompile(`\b\d{10}\b`) // 10-digit NUBAN
	multiSpacesRe   = regexp.MustCompile(`\s+`)
	specialCharsRe  = regexp.MustCompile(`[/*\\|#_~;:]+`)
)

// MaskPII replaces sensitive numbers (NUBAN account numbers, phone numbers) with masked equivalents.
func MaskPII(input string) string {
	// Mask phone numbers: 08031234567 -> 0803****567
	masked := phoneRe.ReplaceAllStringFunc(input, func(s string) string {
		if len(s) >= 10 {
			return s[:4] + "****" + s[len(s)-3:]
		}
		return "****"
	})

	// Mask 10-digit NUBAN account numbers: 0123456789 -> 0123****89
	masked = accountNumRe.ReplaceAllStringFunc(masked, func(s string) string {
		return s[:4] + "****" + s[len(s)-2:]
	})

	return masked
}

// CleanMemo strips clearinghouse tags, terminal codes, session IDs, and excessive punctuation.
func CleanMemo(rawMemo string) string {
	if rawMemo == "" {
		return ""
	}

	cleaned := rawMemo

	// 1. Remove NIP/CR, NIP/DR, TRF/TO noise
	cleaned = nipCrDrRe.ReplaceAllString(cleaned, " ")

	// 2. Remove long NIBSS session numbers
	cleaned = sessionIDRe.ReplaceAllString(cleaned, " ")

	// 3. Remove POS terminal references
	cleaned = posTerminalRe.ReplaceAllString(cleaned, " ")

	// 4. Remove RRN and reference codes
	cleaned = rrnRe.ReplaceAllString(cleaned, " ")

	// 5. Remove web gateway prefixes (Paystack *, Flutterwave *)
	cleaned = webPayRe.ReplaceAllString(cleaned, " ")

	// 6. Replace delimiters like slashes, pipes, underscores with spaces
	cleaned = specialCharsRe.ReplaceAllString(cleaned, " ")

	// 7. Remove standalone timestamps embedded in remarks
	cleaned = dateTimeStampRe.ReplaceAllString(cleaned, " ")

	// 8. Collapse spaces and trim
	cleaned = multiSpacesRe.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)

	return cleaned
}

// ExtractTimeWindow attempts to extract hour or time marker from memo or returns a human string.
func ExtractTimeWindow(memo string) (string, int) {
	// Check for HH:MM pattern
	timeRe := regexp.MustCompile(`\b([01]?\d|2[0-3]):([0-5]\d)(?::([0-5]\d))?\s*(AM|PM)?\b`)
	matches := timeRe.FindStringSubmatch(memo)
	if len(matches) > 2 {
		return matches[0], -1 // Raw time string found
	}

	// Late night markers
	lower := strings.ToLower(memo)
	if strings.Contains(lower, "midnight") || strings.Contains(lower, "late delivery") || strings.Contains(lower, "club") || strings.Contains(lower, "lounge") {
		return "Late Night (10PM - 3AM)", 23
	}

	return "", -1
}
