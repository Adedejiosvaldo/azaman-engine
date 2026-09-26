package analyzer

import (
	"fmt"
	"math"
	"strings"

	"azaman-engine/internal/models"
)

// AnalyzeInsights tallies Nigeria-specific spending patterns and raises plain-language warnings.
func AnalyzeInsights(txs []models.DecodedTransaction, internal models.InternalTransfersReport,
	recurring []models.RecurringPayment, summary models.AuditSummary) models.NigeriaInsights {
	in := models.NigeriaInsights{
		RecurringPayments: recurring,
		Warnings:          []string{},
	}

	for _, tx := range txs {
		if tx.Flags.IsReversal {
			continue
		}
		switch {
		case tx.Category == "Betting & Gaming" && tx.Debit > 0:
			in.BettingSpend += tx.Debit
			in.BettingCount++
		case tx.Category == "Betting & Gaming" && tx.Credit > 0:
			in.BettingWinnings += tx.Credit
		case tx.Flags.IsLoan && tx.Credit > 0:
			in.LoanDisbursements += tx.Credit
		case tx.Flags.IsLoan && tx.Debit > 0:
			in.LoanRepayments += tx.Debit
		case tx.Debit <= 0:
			continue
		case tx.Category == "Airtime & Utilities" && isAirtime(tx):
			in.AirtimeDataSpend += tx.Debit
		case tx.Category == "Ajo & Thrift":
			in.AjoContributions += tx.Debit
		case tx.Category == "Religious Giving":
			in.ReligiousGiving += tx.Debit
		case tx.Category == "Black Tax (Family Support)":
			in.BlackTax += tx.Debit
		case tx.Category == "Subscriptions & TV":
			in.SubscriptionsTV += tx.Debit
		}
	}

	income := summary.TotalInflow - in.BettingWinnings
	if income > 0 {
		in.DebtServiceRatioPct = round2(in.LoanRepayments / income * 100)
		in.SavingsRatePct = round2((internal.NetSaved + in.AjoContributions) / income * 100)
	}

	sym := currencySymbol(summary.Currency)
	if in.BettingSpend > 0 {
		net := in.BettingWinnings - in.BettingSpend
		in.Warnings = append(in.Warnings, fmt.Sprintf("Betting: %s%s staked across %d bets (net %s%s%s).",
			sym, money(in.BettingSpend), in.BettingCount, sign(net), sym, money(math.Abs(net))))
	}
	if in.DebtServiceRatioPct > 30 {
		in.Warnings = append(in.Warnings, fmt.Sprintf("Loan repayments take %.1f%% of income; above 30%% is a debt-trap risk.", in.DebtServiceRatioPct))
	}
	if income > 0 && in.AirtimeDataSpend/income > 0.05 {
		in.Warnings = append(in.Warnings, fmt.Sprintf("Airtime & data is %.1f%% of income; a monthly bundle is usually cheaper than daily top-ups.", in.AirtimeDataSpend/income*100))
	}
	if in.BlackTax > 0 && income > 0 {
		in.Warnings = append(in.Warnings, fmt.Sprintf("Recurring family support (black tax) is %.1f%% of income; budget it as a fixed cost.", in.BlackTax/income*100))
	}
	if in.SavingsRatePct < 10 && income > 0 {
		in.Warnings = append(in.Warnings, fmt.Sprintf("Savings rate is %.1f%%; aim for at least 10%%.", in.SavingsRatePct))
	}

	in.BettingSpend = round2(in.BettingSpend)
	in.BettingWinnings = round2(in.BettingWinnings)
	in.AirtimeDataSpend = round2(in.AirtimeDataSpend)
	in.LoanDisbursements = round2(in.LoanDisbursements)
	in.LoanRepayments = round2(in.LoanRepayments)
	in.AjoContributions = round2(in.AjoContributions)
	in.ReligiousGiving = round2(in.ReligiousGiving)
	in.BlackTax = round2(in.BlackTax)
	in.SubscriptionsTV = round2(in.SubscriptionsTV)
	return in
}

func isAirtime(tx models.DecodedTransaction) bool {
	return !strings.Contains(tx.Merchant, "Power") && !strings.Contains(tx.Merchant, "Electric")
}

// Nigeria Tax Act 2025 personal income tax bands (effective 1 January 2026), annual naira.
var pitBands = []struct {
	upTo float64
	rate float64
}{
	{800_000, 0.00},
	{3_000_000, 0.15},
	{12_000_000, 0.18},
	{25_000_000, 0.21},
	{50_000_000, 0.23},
	{math.Inf(1), 0.25},
}

const (
	rentReliefRate = 0.20
	rentReliefCap  = 500_000
)

// EstimateTax annualizes statement income and computes indicative PIT under the Nigeria Tax Act 2025.
// Income = real credits (excludes self-transfers, savings withdrawals, reversals, loans, betting wins).
func EstimateTax(txs []models.DecodedTransaction, periodDays float64, currency string) models.TaxEstimate {
	est := models.TaxEstimate{
		Regime: "Nigeria Tax Act 2025 (PIT, effective 2026)",
		Note:   "Indicative only. Assumes all non-internal credits are taxable income; excludes pension, NHF and NHIS deductions. Confirm with a tax adviser.",
	}
	if currency != "" && currency != "NGN" {
		est.Note = "Tax estimate only computed for NGN statements."
		return est
	}
	if periodDays < 1 {
		periodDays = 1
	}
	factor := 365.0 / periodDays

	var income, deductibles, rent float64
	for _, tx := range txs {
		switch {
		case tx.IsEarning() && !tx.Flags.IsLoan && tx.Category != "Betting & Gaming":
			income += tx.Credit
		case tx.IsSpend() && tx.Flags.IsTaxDeductible:
			deductibles += tx.Debit
		case tx.IsSpend() && tx.Category == "Rent & Housing":
			rent += tx.Debit
		}
	}

	est.AnnualizedIncome = round2(income * factor)
	est.AnnualizedDeductibles = round2(deductibles * factor)
	est.RentRelief = round2(math.Min(rent*factor*rentReliefRate, rentReliefCap))
	chargeable := math.Max(0, est.AnnualizedIncome-est.AnnualizedDeductibles-est.RentRelief)
	est.ChargeableIncome = round2(chargeable)

	var tax, lower float64
	for _, b := range pitBands {
		if chargeable <= lower {
			break
		}
		tax += (math.Min(chargeable, b.upTo) - lower) * b.rate
		lower = b.upTo
	}
	est.EstimatedAnnualTax = round2(tax)
	if est.AnnualizedIncome > 0 {
		est.EffectiveRatePct = round2(tax / est.AnnualizedIncome * 100)
	}
	return est
}

// AnalyzeBusinessSplit separates business flows (tech vendors, commercial payees, client income)
// from personal ones, giving informal traders a simple P&L.
func AnalyzeBusinessSplit(txs []models.DecodedTransaction) models.BusinessSplit {
	var s models.BusinessSplit
	for _, tx := range txs {
		switch {
		case tx.IsEarning() && tx.Flags.IsBusiness:
			s.BusinessInflow += tx.Credit
		case tx.IsEarning():
			s.PersonalInflow += tx.Credit
		case tx.IsSpend() && (tx.Flags.IsBusiness || tx.Flags.IsTaxDeductible):
			s.BusinessOutflow += tx.Debit
		case tx.IsSpend():
			s.PersonalOutflow += tx.Debit
		}
	}
	s.BusinessNet = round2(s.BusinessInflow - s.BusinessOutflow)
	s.BusinessInflow = round2(s.BusinessInflow)
	s.BusinessOutflow = round2(s.BusinessOutflow)
	s.PersonalInflow = round2(s.PersonalInflow)
	s.PersonalOutflow = round2(s.PersonalOutflow)
	return s
}

func currencySymbol(c string) string {
	switch c {
	case "USD":
		return "$"
	case "GBP":
		return "£"
	case "EUR":
		return "€"
	default:
		return "₦"
	}
}

func money(v float64) string {
	whole := fmt.Sprintf("%.0f", v)
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func sign(v float64) string {
	if v < 0 {
		return "-"
	}
	return "+"
}
