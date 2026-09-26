package analyzer

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"azaman-engine/internal/models"
)

// daysPerMonth is the mean Gregorian month length (365.2425 / 12).
const daysPerMonth = 30.44

// burnDecay is the per-month weight decay used for the inflation-weighted burn: the most recent month
// weighs 1.0, the month before 0.85, and so on. Naira prices move fast, so older months say less about
// what the next month will cost.
const burnDecay = 0.85

// CalculateRunway computes consolidated cashflow metrics, profit margins, and runway duration.
// Internal movements (self-transfers, savings wallets, reversed debits) are excluded from inflow and outflow.
func CalculateRunway(txs []models.DecodedTransaction, periodDays float64) models.AuditSummary {
	var totalInflow, totalOutflow float64
	var currentBalance float64
	var latest time.Time
	var minDate, maxDate time.Time
	var currency = "NGN"
	haveBalance := false
	monthly := make(map[string]float64)

	for _, tx := range txs {
		if tx.Currency != "" {
			currency = tx.Currency
		}

		if tx.IsEarning() {
			totalInflow += tx.Credit
		} else if tx.IsSpend() {
			totalOutflow += tx.Debit
			if !tx.ParsedDate.IsZero() {
				monthly[tx.ParsedDate.Format("2006-01")] += tx.Debit
			}
		}

		// Track date range
		if !tx.ParsedDate.IsZero() {
			if minDate.IsZero() || tx.ParsedDate.Before(minDate) {
				minDate = tx.ParsedDate
			}
			if maxDate.IsZero() || tx.ParsedDate.After(maxDate) {
				maxDate = tx.ParsedDate
			}
		}

		// Closing balance = balance on the most recent row. Statements can be newest-first,
		// so use the date rather than row order; ties keep the later row.
		if tx.Balance != 0 && (!haveBalance || !tx.ParsedDate.Before(latest)) {
			currentBalance = tx.Balance
			latest = tx.ParsedDate
			haveBalance = true
		}
	}

	netRetained := totalInflow - totalOutflow
	if !haveBalance {
		// No balance column: net retained is the best proxy
		currentBalance = netRetained
	}

	var profitMarginPct float64
	if totalInflow > 0 {
		profitMarginPct = (netRetained / totalInflow) * 100.0
	}

	if periodDays < 1.0 {
		periodDays = 1.0
	}

	avgDailyBurn := totalOutflow / periodDays
	avgMonthlyBurn := avgDailyBurn * daysPerMonth
	weightedBurn := weightedMonthlyBurn(monthly, minDate, maxDate, avgMonthlyBurn)

	var runwayMonths float64
	if weightedBurn > 0 {
		if currentBalance > 0 {
			runwayMonths = currentBalance / weightedBurn
		} else if netRetained > 0 {
			runwayMonths = 999.0 // Self-sustaining
		}
	} else if currentBalance > 0 {
		runwayMonths = 999.0
	}

	periodStart := ""
	periodEnd := ""
	if !minDate.IsZero() {
		periodStart = minDate.Format("2006-01-02")
	}
	if !maxDate.IsZero() {
		periodEnd = maxDate.Format("2006-01-02")
	}

	return models.AuditSummary{
		TotalInflow:         round2(totalInflow),
		TotalOutflow:        round2(totalOutflow),
		NetRetained:         round2(netRetained),
		ProfitMarginPct:     round2(profitMarginPct),
		RunwayMonths:        math.Round(runwayMonths*10) / 10,
		Currency:            currency,
		TransactionCount:    len(txs),
		PeriodStart:         periodStart,
		PeriodEnd:           periodEnd,
		CurrentBalance:      round2(currentBalance),
		AvgDailyBurn:        round2(avgDailyBurn),
		AvgMonthlyBurn:      round2(avgMonthlyBurn),
		WeightedMonthlyBurn: round2(weightedBurn),
	}
}

// weightedMonthlyBurn returns an exponentially weighted average of complete calendar months
// (partial first/last months are skipped). Falls back to the flat average when fewer than 2 full months exist.
func weightedMonthlyBurn(monthly map[string]float64, minDate, maxDate time.Time, fallback float64) float64 {
	if minDate.IsZero() || maxDate.IsZero() {
		return fallback
	}
	first := time.Date(minDate.Year(), minDate.Month(), 1, 0, 0, 0, 0, time.UTC)
	if minDate.Day() != 1 {
		first = first.AddDate(0, 1, 0)
	}
	last := time.Date(maxDate.Year(), maxDate.Month(), 1, 0, 0, 0, 0, time.UTC)
	if maxDate.AddDate(0, 0, 1).Month() == maxDate.Month() {
		last = last.AddDate(0, -1, 0) // statement ends mid-month
	}

	var months []string
	for m := first; !m.After(last); m = m.AddDate(0, 1, 0) {
		months = append(months, m.Format("2006-01"))
	}
	if len(months) < 2 {
		return fallback
	}

	var weighted, weights float64
	w := 1.0
	for i := len(months) - 1; i >= 0; i-- {
		weighted += monthly[months[i]] * w
		weights += w
		w *= burnDecay
	}
	return weighted / weights
}

// CBN Guide to Charges / Nigeria Tax Act limits used to spot disputable overcharges.
const (
	emtlAmount        = 50.0    // ₦50 per electronic receipt
	emtlThreshold     = 10000.0 // only on receipts of ₦10,000 and above
	maxNIPFeeWithVAT  = 53.75   // ₦50 top tier + 7.5% VAT
	maxUSSDSessionFee = 6.98    // NCC/CBN per-session USSD charge
	maxCardFeePerQtr  = 50.0    // ₦50 per quarter on naira debit cards
	feeToleranceNaira = 0.01
)

// AnalyzeBankCharges breaks bank fees into Nigerian fee types and flags charges above regulatory limits.
func AnalyzeBankCharges(txs []models.DecodedTransaction) models.BankChargesReport {
	report := models.BankChargesReport{
		Overcharges:  []models.Overcharge{},
		Transactions: []models.DecodedTransaction{},
	}

	var emtlIdx []int
	cardByQuarter := make(map[string][]int)
	qualifyingReceipts := 0

	for i := range txs {
		tx := &txs[i]
		if tx.Credit >= emtlThreshold && !tx.Flags.IsReversal && !tx.Flags.IsSavings && (tx.Currency == "NGN" || tx.Currency == "") {
			qualifyingReceipts++
		}
		if !tx.Flags.IsBankFee && tx.Category != "Bank Fees & Charges" {
			continue
		}

		fee := tx.Debit
		if fee <= 0 && tx.Amount < 0 {
			fee = math.Abs(tx.Amount)
		}
		if fee <= 0 {
			continue
		}

		tx.FeeType = classifyFee(strings.ToLower(tx.RawDescription + " " + tx.CleanMemo + " " + tx.Merchant))
		report.TotalCharges += fee
		report.Count++

		switch tx.FeeType {
		case "SMS Alert":
			report.SMSAlerts += fee
		case "EMTL":
			report.EMTL += fee
			report.StampDuty += fee
			emtlIdx = append(emtlIdx, i)
		case "Stamp Duty":
			report.StampDuty += fee
		case "Cybersecurity Levy":
			report.CyberLevy += fee
		case "USSD":
			// Not checked against the ₦6.98 NCC rate: that is per 20-second session, and one
			// transaction can span several sessions.
			report.USSDCharges += fee
		case "Card Maintenance":
			report.MaintenanceFees += fee
			if !tx.ParsedDate.IsZero() {
				q := fmt.Sprintf("%d-Q%d", tx.ParsedDate.Year(), (int(tx.ParsedDate.Month())-1)/3+1)
				cardByQuarter[q] = append(cardByQuarter[q], i)
			}
		case "Account Maintenance":
			report.MaintenanceFees += fee
		case "VAT":
			report.VATOnFees += fee
		case "Transfer Fee":
			report.TransferFees += fee
			if fee > maxNIPFeeWithVAT+feeToleranceNaira {
				addOvercharge(&report, tx, "Transfer Fee", fee, maxNIPFeeWithVAT, "NIP transfer fee is capped at ₦50 + 7.5% VAT")
			}
		default:
			report.OtherFees += fee
		}
	}

	// EMTL: more ₦50 levies than qualifying ₦10k+ receipts means some were charged wrongly.
	// Levies beyond the first `qualifyingReceipts` (chronologically) are flagged.
	if extra := len(emtlIdx) - qualifyingReceipts; extra > 0 {
		sort.Slice(emtlIdx, func(a, b int) bool { return txs[emtlIdx[a]].ParsedDate.Before(txs[emtlIdx[b]].ParsedDate) })
		for _, i := range emtlIdx[len(emtlIdx)-extra:] {
			addOvercharge(&report, &txs[i], "EMTL", txs[i].Debit, 0,
				"EMTL (₦50) applies only to receipts of ₦10,000+; more levies than qualifying receipts")
		}
	}
	for i := range emtlIdx {
		tx := &txs[emtlIdx[i]]
		if tx.Debit > emtlAmount+feeToleranceNaira {
			addOvercharge(&report, tx, "EMTL", tx.Debit, emtlAmount, "EMTL is a flat ₦50")
		}
	}

	// Card maintenance: total per quarter above ₦50
	for q, idxs := range cardByQuarter {
		var sum float64
		for _, i := range idxs {
			sum += txs[i].Debit
		}
		if sum > maxCardFeePerQtr+feeToleranceNaira {
			last := &txs[idxs[len(idxs)-1]]
			addOvercharge(&report, last, "Card Maintenance", sum, maxCardFeePerQtr,
				"Naira debit card maintenance is capped at ₦50 per quarter ("+q+")")
		}
	}

	for i := range txs {
		if txs[i].FeeType != "" {
			report.Transactions = append(report.Transactions, txs[i])
		}
	}

	report.TotalCharges = round2(report.TotalCharges)
	report.TotalOvercharge = round2(report.TotalOvercharge)
	return report
}

func addOvercharge(r *models.BankChargesReport, tx *models.DecodedTransaction, feeType string, charged, allowed float64, rule string) {
	excess := round2(charged - allowed)
	if excess <= 0 {
		return
	}
	tx.Flags.IsOvercharge = true
	r.TotalOvercharge += excess
	r.Overcharges = append(r.Overcharges, models.Overcharge{
		FeeType: feeType,
		Date:    tx.Date,
		Charged: round2(charged),
		Allowed: allowed,
		Excess:  excess,
		Rule:    rule,
	})
}

func classifyFee(lower string) string {
	switch {
	case strings.Contains(lower, "emtl") || strings.Contains(lower, "electronic money transfer levy"):
		return "EMTL"
	case strings.Contains(lower, "stamp"):
		return "Stamp Duty"
	case strings.Contains(lower, "cyber"):
		return "Cybersecurity Levy"
	case strings.Contains(lower, "sms") || strings.Contains(lower, "alert"):
		return "SMS Alert"
	case strings.Contains(lower, "ussd"):
		return "USSD"
	case strings.Contains(lower, "card"):
		return "Card Maintenance"
	case strings.Contains(lower, "maintenance") || strings.Contains(lower, "commission on turnover") || strings.Contains(lower, "token fee"):
		return "Account Maintenance"
	case strings.HasPrefix(lower, "vat") || strings.Contains(lower, " vat") || strings.Contains(lower, "value added tax"):
		return "VAT"
	case strings.Contains(lower, "nip") || strings.Contains(lower, "transfer") || strings.Contains(lower, "trf"):
		return "Transfer Fee"
	default:
		return "Other"
	}
}
