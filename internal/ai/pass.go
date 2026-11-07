package ai

import (
	"math"
	"sort"

	"azaman-engine/internal/models"
	"azaman-engine/internal/shazam"
)

// Classifier is what the decision pass needs from an AI backend (the Laya sidecar, or a fake in tests).
type Classifier interface {
	IsAvailable() bool
	ClassifyBatch(items []LayaMemoItem) (map[string]LayaDecision, error)
}

// PassStats says what the AI pass did, for logs and the memo evaluation.
type PassStats struct {
	Candidates int // distinct unsure memos that could have been sent
	Sent       int // memos actually sent
	Changed    int // transactions whose category the AI changed
}

// maxAIItems caps how many distinct memos one audit sends. The ones with the most money go first.
const maxAIItems = 25

// ApplyDecisions sends unsure spending memos to the AI and applies its answers where it is more
// confident than the rules. Rows whose type is already certain (fees, savings, loans, own
// transfers) and plain transfers to people are never sent. What is sent is cleaned first: no
// account numbers, no bank names, no account holder name (see CleanForAI).
func ApplyDecisions(txs []models.DecodedTransaction, client Classifier, holder string) PassStats {
	var stats PassStats
	if client == nil || !client.IsAvailable() {
		return stats
	}

	// Group unsure rows by their cleaned text, so one answer covers every repeat, and add up the
	// money behind each so the biggest unknowns are asked about first.
	type group struct {
		item  LayaMemoItem
		total float64
	}
	groups := map[string]*group{}
	for _, tx := range txs {
		if tx.Confidence >= 0.90 || tx.Amount >= 0 || !Eligible(tx) {
			continue
		}
		text := CleanForAI(Text(tx), holder)
		if text == "" {
			continue
		}
		g, ok := groups[text]
		if !ok {
			g = &group{item: LayaMemoItem{
				ID:       tx.ID,
				Memo:     text,
				Amount:   math.Abs(tx.Amount),
				Hour:     KnownHour(tx),
				IsCredit: tx.Credit > 0,
				Currency: tx.Currency,
			}}
			groups[text] = g
		}
		g.total += math.Abs(tx.Amount)
	}
	stats.Candidates = len(groups)
	ranked := make([]*group, 0, len(groups))
	for _, g := range groups {
		ranked = append(ranked, g)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].total != ranked[j].total {
			return ranked[i].total > ranked[j].total
		}
		return ranked[i].item.Memo < ranked[j].item.Memo
	})
	if len(ranked) > maxAIItems {
		ranked = ranked[:maxAIItems]
	}
	items := make([]LayaMemoItem, len(ranked))
	for i, g := range ranked {
		items[i] = g.item
	}
	stats.Sent = len(items)
	if len(items) == 0 {
		return stats
	}

	decisions, err := client.ClassifyBatch(items)
	if err != nil {
		return stats
	}
	byText := make(map[string]LayaDecision, len(items))
	for _, item := range items {
		if dec, ok := decisions[item.ID]; ok {
			byText[item.Memo] = dec
		}
	}

	for i := range txs {
		tx := &txs[i]
		if tx.Amount >= 0 {
			continue
		}
		dec, ok := byText[CleanForAI(Text(*tx), holder)]
		// Only override when the AI is more confident than the rules, and never for rows
		// whose type is already certain (fees, savings, loans).
		if !ok || !Eligible(*tx) || dec.Confidence <= tx.Confidence {
			continue
		}
		before := tx.Category
		apply(tx, dec)
		if tx.Category != before {
			stats.Changed++
		}
	}
	return stats
}

func apply(tx *models.DecodedTransaction, dec LayaDecision) {
	if tx.Counterparty != "" {
		tx.DecisionBasis = "ai" // person scoring leaves AI-decided rows alone
	}
	if dec.Category != "" {
		tx.Category = dec.Category
	}
	if dec.Classification != "" {
		tx.Classification = dec.Classification
	}
	if dec.Merchant != "" && tx.Confidence < 0.85 {
		tx.Merchant = dec.Merchant
	}
	if dec.Confidence > tx.Confidence {
		tx.Confidence = dec.Confidence
	}
	// The AI's "silent leak" is about discretionary spend, not time of day: only count it as a
	// midnight leak when the row's time is known to be late.
	if h := KnownHour(*tx); dec.IsSilentLeak && h != nil && (*h >= 22 || *h <= 3) {
		tx.Flags.IsMidnightLeak = true
	}
	if dec.IsUrgent2k {
		tx.Flags.IsUrgent2k = true
	}
	if dec.IsTaxDeductible {
		tx.Flags.IsTaxDeductible = true
	}
	// "Bought, not given": a payment to a person for goods or a service. Only a vague category is
	// replaced; a specific one ("hair making" -> Personal Care) already says what was bought.
	vague := tx.Category == "Family & Favors" || tx.Category == "Personal Discretionary" || tx.Category == ""
	if dec.IsPurchase && tx.Counterparty != "" && tx.Debit > 0 && vague {
		tx.Category = shazam.VendorCategory
		tx.Classification = "Personal Purchase"
		tx.Flags.IsUrgent2k = false
	}
}

// Eligible reports whether the AI may decide this row: not a fee, savings, loan or own transfer,
// and not a plain transfer to a person (only the user knows what that was for). A transfer to a
// business is allowed: its name ("Golden Crust Bakery Ltd") says what was bought, and a company
// name is not personal data.
func Eligible(tx models.DecodedTransaction) bool {
	return !tx.Flags.IsBankFee && !tx.Flags.IsSavings && !tx.Flags.IsLoan && !tx.Flags.IsSelfTransfer &&
		(tx.Counterparty == "" || tx.Purpose != "" || tx.Flags.IsBusiness)
}

// KnownHour returns the transaction's hour of day, or nil for date-only rows (parsed as 00:00:00).
func KnownHour(tx models.DecodedTransaction) *int {
	t := tx.ParsedDate
	if t.IsZero() || (t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0) {
		return nil
	}
	h := t.Hour()
	return &h
}

// Text is what the AI reads, before cleaning: the typed purpose of a person-to-person transfer
// when there is one (it carries the meaning; the name only adds noise), the business name for a
// transfer to a business, otherwise the memo.
func Text(tx models.DecodedTransaction) string {
	switch {
	case tx.Purpose != "":
		return tx.Purpose
	case tx.Counterparty != "" && tx.Flags.IsBusiness:
		return tx.Counterparty
	}
	return tx.CleanMemo
}
