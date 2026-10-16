package analyzer

import "azaman-engine/internal/models"

// UncategorizedCategory holds debits the engine can't place with reasonable confidence.
const UncategorizedCategory = "Uncategorized"

// MarkUncategorized replaces weak default guesses with an honest "Uncategorized":
//   - transfers to a person with no typed purpose whom person scoring couldn't settle
//     (previously defaulted to Family & Favors, inflating Urgent 2k)
//   - memos no rule, heuristic or AI recognised (previously defaulted to Personal Discretionary)
//
// Uncategorized rows still count in total spending, but never in leak detectors. The user can
// sort them with the dashboard's Fix menu, or leave them.
func MarkUncategorized(txs []models.DecodedTransaction) models.UncategorizedReport {
	var r models.UncategorizedReport
	for i := range txs {
		tx := &txs[i]
		if !tx.IsSpend() || tx.Flags.IsBankFee || tx.Confidence >= 0.9 {
			continue
		}
		unknownPerson := tx.DecisionBasis == "name" && tx.Category == "Family & Favors"
		unknownMemo := tx.Category == "Personal Discretionary" && tx.Confidence <= 0.7
		if !unknownPerson && !unknownMemo {
			continue
		}
		tx.Category = UncategorizedCategory
		tx.Classification = "Needs review"
		tx.Flags.IsUrgent2k = false
		tx.Flags.IsMidnightLeak = false
		r.Count++
		r.Total += tx.Debit
	}
	r.Total = round2(r.Total)
	return r
}
