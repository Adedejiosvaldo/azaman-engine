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
	toNameRe   = regexp.MustCompile(`(?i)(?:TRANSFER\s+TO|SENT\s+TO|PAYMENT\s+TO|TRF\s+TO|NIP/DR/[^/]+/)\s*([A-Za-z\s]{3,30})`)
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

	// Date-only statements parse to exactly 00:00:00; treat that as "time unknown", not midnight.
	hasTime := !raw.ParsedDate.IsZero() &&
		(raw.ParsedDate.Hour() != 0 || raw.ParsedDate.Minute() != 0 || raw.ParsedDate.Second() != 0)

	// 1. Try matching against our curated merchant dictionary. Person-to-person transfers are split
	// into counterparty and purpose first: the purpose decides this payment, the name only matches
	// merchant rules (so "Mama Tolu Stores" is not "family" and "Grace Food Ventures" is not "food").
	counterparty, purpose := SplitMemo(raw.Description)
	decoded.Counterparty, decoded.Purpose = counterparty, MaskPII(purpose)
	rule, basis, purchaseGuess := selectRule(cleaned, counterparty, purpose, isCredit)
	decoded.DecisionBasis = basis

	if purchaseGuess {
		// Money given uses a small vocabulary (upkeep, allowance, feeding...); anything else the
		// sender typed ("cow leg", "2 bags", "balance for order") is most likely a purchase.
		decoded.Merchant = titleCase(counterparty) + " (Vendor?)"
		decoded.Category = VendorCategory
		decoded.Classification = "Personal Purchase"
		decoded.Confidence = 0.75 // below 0.9: Laya and the review panel may still correct it
	} else if rule != nil {
		decoded.Merchant = rule.Merchant
		decoded.Category = rule.Category
		decoded.Classification = rule.Classification
		decoded.Confidence = rule.Confidence
		decoded.Flags.IsBankFee = rule.IsFee
		decoded.Flags.IsSavings = rule.IsSavings
		decoded.Flags.IsLoan = rule.IsLoan
		decoded.Flags.IsSelfTransfer = rule.IsSelfTransfer
		// Loan disbursements and savings withdrawals are cash in, but not income.
		decoded.Flags.IsIncome = rule.IsIncome || (isCredit && !rule.IsLoan && !rule.IsSavings && !rule.IsSelfTransfer)
		decoded.Flags.IsTaxDeductible = rule.IsTechVendor
		decoded.Flags.IsBusiness = rule.IsTechVendor || (rule.IsIncome && isCredit)
		if rule.IsLoan && isCredit {
			decoded.Category = "Loan Disbursement"
			decoded.Classification = "Borrowed Funds"
		}
		if basis == "memo" && counterparty != "" {
			// Keep who it went to visible: "Tunde (Transport Fare)"
			decoded.Merchant = titleCase(counterparty) + " (" + rule.Merchant + ")"
		}

		// Known late-night merchant: only assume midnight when the statement has no time of day
		if rule.IsLateNight && !isCredit && !hasTime {
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
	if hasTime {
		hour := raw.ParsedDate.Hour()
		if hour >= 22 || hour <= 3 {
			// Midnight window: 22:00 - 03:59
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
	isNonFavor := decoded.Flags.IsSavings || decoded.Flags.IsLoan || decoded.Flags.IsBankFee || decoded.Flags.IsSelfTransfer
	if !isCredit && !isNonFavor && (currency == "NGN" || currency == "") && absAmount >= 1000 && absAmount <= 15000 {
		lowerDesc := strings.ToLower(raw.Description)
		lowerClean := strings.ToLower(cleaned)
		// Generic transfer wording only counts for person-to-person rows; vendors, businesses and
		// POS agents paid by transfer are purchases, not favors. An explicit "urgent" memo always counts.
		personal := decoded.Category == "Family & Favors" || decoded.Category == "" ||
			decoded.Category == "Uncategorized" || decoded.Category == "Personal Discretionary"
		explicit := strings.Contains(lowerClean, "urgent") || strings.Contains(lowerDesc, "urgent")
		if explicit || (personal && (decoded.Category == "Family & Favors" ||
			decoded.Classification == "Micro-Transfer (Urgent 2k)" ||
			strings.Contains(lowerClean, "transfer to") ||
			strings.Contains(lowerDesc, "nip/dr") ||
			strings.Contains(lowerClean, "favors") ||
			strings.Contains(lowerClean, "upkeep") ||
			ContainsWord(lowerClean, "mum") ||
			strings.Contains(lowerClean, "family"))) {
			decoded.Flags.IsUrgent2k = true
			decoded.Classification = "Micro-Transfer (Urgent 2k)"
			if decoded.Category == "Uncategorized" || decoded.Category == "" {
				decoded.Category = "Family & Favors"
			}
		}
	}

	// Urgent 2k is ₦1,000-₦15,000; smaller or larger person-to-person transfers are plain transfers.
	if decoded.Classification == "Micro-Transfer (Urgent 2k)" && !decoded.Flags.IsUrgent2k {
		decoded.Classification = "Personal Transfer"
	}

	return decoded
}

// selectRule picks the dictionary rule for a memo. For person-to-person transfers the typed
// purpose is checked first; unmatched purpose text on a debit is reported as a purchase guess.
// Without a purpose, only merchant rules are matched against the counterparty's name.
func selectRule(cleaned, counterparty, purpose string, isCredit bool) (rule *MerchantRule, basis string, purchaseGuess bool) {
	if counterparty == "" {
		return FindRuleByKeyword(cleaned), "", false
	}
	if purpose != "" {
		if r := FindRuleByKeyword(CleanMemo(purpose)); r != nil {
			return r, "memo", false
		}
		if !isCredit {
			return nil, "memo", true
		}
	}
	return FindNameRule(counterparty), "name", false
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
	if containsAnyWord(lowerClean, "fee", "fees", "charge", "charges", "duty", "tax", "levy", "vat") {
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
			"limited", "plc", "nig", "ent", "co", "pg", "pay", "solutions", "plaza", "agency",
			"electrical", "electricals", "clinic", "hospital", "hotel", "suites",
		}
		for _, b := range businessSuffixes {
			if matchPattern(lowerName, b) {
				d.Flags.IsBusiness = true
				d.Merchant = titleCase(name)
				d.Category = "Commercial & Business"
				d.Classification = "Operating Expense"
				d.Confidence = 0.89
				return
			}
		}

		// 3. Recipient banks with Moniepoint are overwhelmingly traders and market vendors
		// (business accounts and POS terminals), even when the account is in a personal name.
		if isVendorBank(rawMemo) {
			d.Merchant = titleCase(name) + " (Vendor)"
			d.Category = VendorCategory
			d.Classification = "Personal Purchase"
			d.Confidence = 0.9
			return
		}

		// 4. Genuine Family & Favors transfer
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

// VendorCategory holds payments to traders and market sellers who use personal-name accounts.
const VendorCategory = "Market & Small Vendors"

// vendorBanks are recipient banks whose personal-name accounts are mostly traders and POS merchants.
var vendorBanks = []string{"monie point", "moniepoint"}

// isVendorBank checks the bank segment of "Transfer to NAME | BANK | ACCOUNT" style memos.
func isVendorBank(rawMemo string) bool {
	parts := strings.Split(rawMemo, "|")
	if len(parts) < 2 {
		return false
	}
	bank := strings.ToLower(parts[1])
	for _, b := range vendorBanks {
		if strings.Contains(bank, b) {
			return true
		}
	}
	return false
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

func containsAnyWord(lower string, words ...string) bool {
	for _, w := range words {
		if ContainsWord(lower, w) {
			return true
		}
	}
	return false
}
