package analyzer

import (
	"azaman-engine/internal/models"
)

// Levies the law puts on every account. They count as leaks (worth seeing) but not against the
// grade: nobody can avoid them. An overcharged levy is a real leak and is scored.
var statutoryLevies = map[string]bool{"EMTL": true, "Stamp Duty": true, "Cybersecurity Levy": true}

func isStatutoryLevy(tx models.DecodedTransaction) bool {
	return statutoryLevies[tx.FeeType] && !tx.Flags.IsOvercharge
}

// mainCurrency returns the currency with the most transactions, and whether any other appears.
// Rows without a currency count as NGN, as in CalculateRunway.
func mainCurrency(txs []models.DecodedTransaction) (string, bool) {
	counts := map[string]int{}
	for _, tx := range txs {
		c := tx.Currency
		if c == "" {
			c = "NGN"
		}
		counts[c]++
	}
	best := "NGN"
	for c, n := range counts {
		if n > counts[best] || (n == counts[best] && c < best) {
			best = c
		}
	}
	return best, len(counts) > 1
}

func onlyCurrency(txs []models.DecodedTransaction, currency string) []models.DecodedTransaction {
	out := make([]models.DecodedTransaction, 0, len(txs))
	for _, tx := range txs {
		c := tx.Currency
		if c == "" {
			c = "NGN"
		}
		if c == currency {
			out = append(out, tx)
		}
	}
	return out
}

// gradeSubset grades part of an audit (one currency, or one month) exactly as the full audit is
// graded. txs must already have been through the GenerateAudit pre-passes.
func gradeSubset(txs []models.DecodedTransaction, leakIDs map[string]float64,
	recurring []models.RecurringPayment) models.FinancialGrade {
	periodDays := EstimatePeriodDays(txs)
	summary := CalculateRunway(txs, periodDays)

	var internal models.InternalTransfersReport
	for _, tx := range txs {
		if !tx.Flags.IsSavings {
			continue
		}
		internal.SavingsDeposits += tx.Debit
		internal.SavingsWithdrawal += tx.Credit
	}
	internal.NetSaved = round2(internal.SavingsDeposits - internal.SavingsWithdrawal)

	insights := AnalyzeInsights(txs, internal, recurring, summary)
	mix := BuildSpendingMix(txs, leakIDs, internal.NetSaved)
	return GradeAudit(summary, insights, mix, periodDays, len(txs))
}

// GradeMonths gives each month in the report its own grade, so people can watch it change.
// Months with nothing to score are left blank.
func GradeMonths(monthly *models.MonthlyReport, txs []models.DecodedTransaction,
	leakIDs map[string]float64, recurring []models.RecurringPayment) {
	byMonth := map[string][]models.DecodedTransaction{}
	for _, tx := range txs {
		if tx.ParsedDate.IsZero() {
			continue
		}
		key := tx.ParsedDate.Format("2006-01")
		byMonth[key] = append(byMonth[key], tx)
	}
	for i := range monthly.Months {
		m := &monthly.Months[i]
		g := gradeSubset(byMonth[m.Month], leakIDs, recurring)
		if g.Grade == "N/A" {
			continue
		}
		m.Grade = g.Grade
		m.Score = g.Score
	}
}
