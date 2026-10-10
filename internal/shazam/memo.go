package shazam

import (
	"regexp"
	"strings"
)

var (
	// OPay / PalmPay / Moniepoint: "Transfer to NAME | BANK | ACCOUNT | purpose"
	pipeMemoRe = regexp.MustCompile(`(?i)^\s*(?:transfer|trf)\s+(?:to|from)\s+([^|]+?)\s*\|\s*[^|]*\|\s*[^|]*(?:\|(.*))?$`)
	// Kuda / generic: "Transfer to NAME - purpose"
	dashMemoRe = regexp.MustCompile(`(?i)^\s*(?:transfer|trf|sent)\s+(?:to|from)\s+([A-Za-z][A-Za-z ,.']{1,60}?)\s+-\s+(.+)$`)
	// NIBSS: "NIP/DR/0123456789/NAME/purpose"
	nipMemoRe = regexp.MustCompile(`(?i)^\s*NIP/(?:DR|CR)/[^/]+/([^/]+)(?:/(.*))?$`)

	purposeNoiseRe = regexp.MustCompile(`^[\d*\s:./-]*$`)
	nameAcctRe     = regexp.MustCompile(`^[A-Za-z ,.'-]+:\s*[\d*]+$`) // "NAME SURNAME:9017467883"
)

// SplitMemo separates a person-to-person transfer narration into the counterparty's name and the
// purpose the sender typed. Both are empty for non-transfer memos (card payments, fees, merchants).
// Keeping them apart lets purpose words decide a payment without the name triggering rules
// ("Grace Food Ventures" is a name, not a food purchase).
func SplitMemo(desc string) (counterparty, purpose string) {
	for _, re := range []*regexp.Regexp{pipeMemoRe, dashMemoRe, nipMemoRe} {
		m := re.FindStringSubmatch(desc)
		if m == nil {
			continue
		}
		counterparty = strings.Join(strings.Fields(m[1]), " ")
		if len(m) > 2 {
			purpose = cleanPurpose(m[2], counterparty)
		}
		return counterparty, purpose
	}
	return "", ""
}

func cleanPurpose(p, counterparty string) string {
	var kept []string
	for _, part := range strings.Split(p, "|") {
		part = strings.Join(strings.Fields(part), " ")
		lower := strings.ToLower(part)
		switch {
		case part == "", purposeNoiseRe.MatchString(part), nameAcctRe.MatchString(part):
			continue
		case lower == "sent", lower == "transfer", lower == "trf", lower == "payment", lower == "nip":
			continue
		// OPay's preset purposes: "Purchase" is pre-selected and sent with transfers to savings apps
		// and relatives alike, so neither says what the money was for.
		case lower == "purchase", lower == "personal transfer":
			continue
		case strings.EqualFold(part, counterparty):
			continue
		}
		kept = append(kept, part)
	}
	return strings.Join(kept, " ")
}
