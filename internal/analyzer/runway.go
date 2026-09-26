package analyzer

import (
	"math"
	"strings"
	"time"

	"azaman-engine/internal/models"
)

// CalculateRunway computes consolidated cashflow metrics, profit margins, and runway duration.
func CalculateRunway(txs []models.DecodedTransaction, periodDays float64) models.AuditSummary {
	var totalInflow, totalOutflow float64
	var currentBalance float64
	var minDate, maxDate time.Time
	var currency = "NGN"

	for i, tx := range txs {
		if tx.Currency != "" {
			currency = tx.Currency
		}

		if tx.Amount > 0 || tx.Credit > 0 {
			totalInflow += tx.Credit
		} else if tx.Amount < 0 || tx.Debit > 0 {
			totalOutflow += tx.Debit
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

		// Keep track of latest balance
		if tx.Balance != 0 {
			currentBalance = tx.Balance
		} else if i == len(txs)-1 && currentBalance == 0 {
			// If no explicit balance column, estimate net retained as proxy
			currentBalance = totalInflow - totalOutflow
		}
	}

	netRetained := totalInflow - totalOutflow

	var profitMarginPct float64
	if totalInflow > 0 {
		profitMarginPct = (netRetained / totalInflow) * 100.0
	}

	if periodDays < 1.0 {
		periodDays = 1.0
	}

	avgDailyBurn := totalOutflow / periodDays
	avgMonthlyBurn := avgDailyBurn * 30.0

	var runwayMonths float64
	if avgMonthlyBurn > 0 {
		if currentBalance > 0 {
			runwayMonths = currentBalance / avgMonthlyBurn
		} else if netRetained > 0 {
			// Sustainable positive retention
			runwayMonths = 999.0 // Self-sustaining
		} else {
			runwayMonths = 0.0
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
		TotalInflow:      math.Round(totalInflow*100) / 100,
		TotalOutflow:     math.Round(totalOutflow*100) / 100,
		NetRetained:      math.Round(netRetained*100) / 100,
		ProfitMarginPct:  math.Round(profitMarginPct*100) / 100,
		RunwayMonths:     math.Round(runwayMonths*10) / 10,
		Currency:         currency,
		TransactionCount: len(txs),
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		CurrentBalance:   math.Round(currentBalance*100) / 100,
		AvgDailyBurn:     math.Round(avgDailyBurn*100) / 100,
		AvgMonthlyBurn:   math.Round(avgMonthlyBurn*100) / 100,
	}
}

// AnalyzeBankCharges categorizes SMS alerts, stamp duties (EMTL), and transfer fees.
func AnalyzeBankCharges(txs []models.DecodedTransaction) models.BankChargesReport {
	report := models.BankChargesReport{
		Transactions: []models.DecodedTransaction{},
	}

	for _, tx := range txs {
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

		report.TotalCharges += fee
		report.Count++
		report.Transactions = append(report.Transactions, tx)

		lower := strings.ToLower(tx.RawDescription + " " + tx.CleanMemo + " " + tx.Merchant)
		if strings.Contains(lower, "sms") || strings.Contains(lower, "alert") {
			report.SMSAlerts += fee
		} else if strings.Contains(lower, "stamp") || strings.Contains(lower, "emtl") || strings.Contains(lower, "levy") {
			report.StampDuty += fee
		} else if strings.Contains(lower, "card") {
			report.MaintenanceFees += fee
		} else if strings.Contains(lower, "nip") || strings.Contains(lower, "transfer") || strings.Contains(lower, "trf") || strings.Contains(lower, "vat") {
			report.TransferFees += fee
		} else {
			report.OtherFees += fee
		}
	}

	return report
}
