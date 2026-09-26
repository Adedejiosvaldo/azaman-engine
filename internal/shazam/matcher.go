package shazam

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"azaman-engine/internal/models"
	"github.com/google/uuid"
)

var (
	// Recipient name extractor for P2P transfers: "TRANSFER TO JOHN DOE" -> "John Doe"
	toNameRe = regexp.MustCompile(`(?i)(?:TRANSFER\s+TO|SENT\s+TO|PAYMENT\s+TO|TRF\s+TO|NIP/DR/[^/]+/)\s*([A-Za-z\s]{3,30})`)
	fromNameRe = regexp.MustCompile(`(?i)(?:TRANSFER\s+FROM|RECEIVED\s+FROM|TRF\s+FROM|NIP/CR/[^/]+/)\s*([A-Za-z\s]{3,30})`)
)

// MatchAndDecode enriches a RawTransaction into a fully categorized DecodedTransaction.
func MatchAndDecode(raw models.RawTransaction) models.DecodedTransaction {
	cleaned := CleanMemo(raw.Description)
	maskedRaw := MaskPII(raw.Description)
	timeWindow, extractedHour := ExtractTimeWindow(raw.Description)

	// Determine transaction amount sign: credit is positive, debit is negative
	var amount float64
	isCredit := false
	if raw.Credit > 0 {
		amount = raw.Credit
		isCredit = true
	} else if raw.Debit > 0 {
		amount = -raw.Debit
	} else {
		// Fallback if raw balance delta or single amount field
		amount = raw.Debit
	}

	absAmount := math.Abs(amount)
	currency := raw.Currency
	if currency == "" {
		currency = "NGN"
	}

	decoded := models.DecodedTransaction{
		ID:             uuid.New().String(),
		Date:           raw.Date,
		ParsedDate:     raw.ParsedDate,
		RawDescription: maskedRaw,
		CleanMemo:      cleaned,
		Amount:         amount,
		Debit:          raw.Debit,
		Credit:         raw.Credit,
		Balance:        raw.Balance,
		Currency:       currency,
		TimeWindow:     timeWindow,
	}

	if decoded.Date == "" && !raw.ParsedDate.IsZero() {
		decoded.Date = raw.ParsedDate.Format("2006-01-02 15:04:05")
	}

	// 1. Try matching against our curated merchant dictionary
	rule := FindRuleByKeyword(cleaned)
	if rule != nil {
		decoded.Merchant = rule.Merchant
		decoded.Category = rule.Category
		decoded.Classification = rule.Classification
		decoded.Confidence = rule.Confidence
		decoded.Flags.IsBankFee = rule.IsFee
		decoded.Flags.IsIncome = rule.IsIncome || isCredit
		decoded.Flags.IsTaxDeductible = rule.IsTechVendor

		// Check if midnight leak candidate
		if rule.IsLateNight && !isCredit {
			decoded.Flags.IsMidnightLeak = true
			if decoded.TimeWindow == "" {
				decoded.TimeWindow = "Late Night (Discretionary)"
			}
		}
	} else {
		// 2. Heuristic fallback based on transaction context
		inferHeuristics(&decoded, cleaned, raw.Description, isCredit, absAmount)
	}

	// 3. Time-based midnight verification if ParsedDate contains time
	if !raw.ParsedDate.IsZero() {
		hour := raw.ParsedDate.Hour()
		if hour >= 22 || hour <= 3 {
			// Midnight window: 10 PM - 3 AM
			if !isCredit && decoded.Classification == "Personal Discretionary" {
				decoded.Flags.IsMidnightLeak = true
			}
			if decoded.TimeWindow == "" {
				decoded.TimeWindow = fmt.Sprintf("%02d:%02d (%s)", hour, raw.ParsedDate.Minute(), raw.ParsedDate.Format("3:04 PM"))
			}
		}
	} else if extractedHour >= 22 || (extractedHour >= 0 && extractedHour <= 3) {
		if !isCredit && (decoded.Classification == "Personal Discretionary" || decoded.Category == "Late Night Food & Lounges") {
			decoded.Flags.IsMidnightLeak = true
		}
	}

	// 4. Urgent 2k detection: Micro-transfers between ₦1,000 and ₦15,000 to individuals/favors
	if !isCredit && (currency == "NGN" || currency == "") && absAmount >= 1000 && absAmount <= 15000 {
		lowerDesc := strings.ToLower(raw.Description)
		lowerClean := strings.ToLower(cleaned)
		if decoded.Category == "Family & Favors" ||
			decoded.Classification == "Micro-Transfer (Urgent 2k)" ||
			strings.Contains(lowerClean, "transfer to") ||
			strings.Contains(lowerDesc, "nip/dr") ||
			strings.Contains(lowerClean, "favors") ||
			strings.Contains(lowerClean, "urgent") ||
			strings.Contains(lowerDesc, "urgent") ||
			strings.Contains(lowerClean, "upkeep") ||
			strings.Contains(lowerClean, "mum") ||
			strings.Contains(lowerClean, "family") {
			decoded.Flags.IsUrgent2k = true
			decoded.Classification = "Micro-Transfer (Urgent 2k)"
			if decoded.Category == "Uncategorized" || decoded.Category == "" {
				decoded.Category = "Family & Favors"
			}
		}
	}

	return decoded
}

// inferHeuristics handles unmatched memos with smart contextual deduction.
func inferHeuristics(d *models.DecodedTransaction, cleaned string, rawMemo string, isCredit bool, absAmount float64) {
	lowerClean := strings.ToLower(cleaned)

	if isCredit {
		d.Flags.IsIncome = true
		d.Classification = "Income / Retainer"
		d.Category = "Income & Retainers"
		d.Confidence = 0.85

		if fromMatches := fromNameRe.FindStringSubmatch(rawMemo); len(fromMatches) > 1 {
			name := strings.TrimSpace(fromMatches[1])
			d.Merchant = "Transfer from " + titleCase(name)
		} else {
			d.Merchant = "Inward Credit / Settlement"
		}
		return
	}

	// Bank charge heuristic
	if strings.Contains(lowerClean, "fee") || strings.Contains(lowerClean, "charge") ||
		strings.Contains(lowerClean, "duty") || strings.Contains(lowerClean, "tax") ||
		strings.Contains(lowerClean, "levy") || strings.Contains(lowerClean, "vat") {
		d.Merchant = "Bank Fee / Surcharge"
		d.Category = "Bank Fees & Charges"
		d.Classification = "Bank Charge"
		d.Flags.IsBankFee = true
		d.Confidence = 0.90
		return
	}

	// P2P vs. Commercial Vendor vs. POS Agent Disambiguation Heuristic
	if toMatches := toNameRe.FindStringSubmatch(rawMemo); len(toMatches) > 1 {
		name := strings.TrimSpace(toMatches[1])
		lowerName := strings.ToLower(name)

		// 1. POS Cashout / Agency Banking check
		if strings.Contains(lowerName, "pos") || strings.Contains(lowerName, "cashout") ||
			strings.Contains(lowerName, "agent") || strings.HasSuffix(lowerName, " man") || lowerName == "bode man" {
			d.Merchant = titleCase(name) + " (POS Agent)"
			d.Category = "POS Cashout & Agency"
			d.Classification = "Cash Withdrawal"
			d.Confidence = 0.91
			return
		}

		// 2. Commercial entity / Sole Proprietorship check
		businessSuffixes := []string{
			"venture", "ventures", "enterprise", "enterprises", "store", "stores",
			"global", "service", "services", "logistics", "pharmacy", "farm", "farms",
			"tech", "salon", "saloon", "motors", "spares", "bakery", "kitchen",
			"buka", "supermarket", "consulting", "holding", "holdings", "ltd",
			"limited", "plc", "nig", "pg", "pay", "solutions", "plaza", "agency",
			"electrical", "electricals", "clinic", "hospital", "hotel", "suites",
		}
		for _, b := range businessSuffixes {
			if strings.Contains(lowerName, b) {
				d.Merchant = titleCase(name)
				d.Category = "Commercial & Business"
				d.Classification = "Operating Expense"
				d.Confidence = 0.89
				return
			}
		}

		// 3. Genuine Family & Favors transfer
		d.Merchant = "Transfer to " + titleCase(name)
		d.Category = "Family & Favors"
		d.Classification = "Micro-Transfer (Urgent 2k)"
		d.Confidence = 0.88
		return
	}

	// Default fallback
	if len(cleaned) > 0 {
		parts := strings.Fields(cleaned)
		if len(parts) > 3 {
			d.Merchant = strings.Join(parts[:3], " ")
		} else {
			d.Merchant = cleaned
		}
	} else {
		d.Merchant = "General Debit"
	}

	d.Category = "Personal Discretionary"
	d.Classification = "Personal Discretionary"
	d.Confidence = 0.70
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, " ")
}
