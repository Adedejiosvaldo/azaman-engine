package analyzer

import (
	"strings"
	"time"

	"azaman-engine/internal/models"
)

// AnalyzeUrgent2k clusters micro-transfers (₦1,000 - ₦15,000) sent to individuals/favors.
func AnalyzeUrgent2k(txs []models.DecodedTransaction, periodDays float64) models.Urgent2kReport {
	report := models.Urgent2kReport{
		Transactions: []models.DecodedTransaction{},
	}

	familyKeywords := []string{"mum", "dad", "mama", "papa", "sister", "brother", "family", "home", "upkeep", "parents", "cousin"}

	for _, tx := range txs {
		if tx.Amount >= 0 || tx.Debit <= 0 {
			continue
		}

		if tx.Flags.IsUrgent2k {
			report.TotalAmount += tx.Debit
			report.Count++
			report.Transactions = append(report.Transactions, tx)

			// Determine if family or errand/favor
			lowerMemo := strings.ToLower(tx.RawDescription + " " + tx.CleanMemo + " " + tx.Merchant)
			isFamily := false
			for _, kw := range familyKeywords {
				if strings.Contains(lowerMemo, kw) {
					isFamily = true
					break
				}
			}

			if isFamily {
				report.FamilyVsErrandSplit.FamilyAmount += tx.Debit
				report.FamilyVsErrandSplit.FamilyCount++
			} else {
				report.FamilyVsErrandSplit.ErrandFavorsAmount += tx.Debit
				report.FamilyVsErrandSplit.ErrandFavorsCount++
			}
		}
	}

	if report.Count > 0 {
		report.AverageTransferSize = report.TotalAmount / float64(report.Count)
	}

	weeks := periodDays / 7.0
	if weeks < 1.0 {
		weeks = 1.0
	}
	report.FrequencyPerWeek = float64(report.Count) / weeks

	return report
}

// EstimatePeriodDays returns days between the oldest and newest transaction dates.
func EstimatePeriodDays(txs []models.DecodedTransaction) float64 {
	if len(txs) == 0 {
		return 30.0 // Default 1 month
	}

	var minDate, maxDate time.Time
	for _, tx := range txs {
		if tx.ParsedDate.IsZero() {
			continue
		}
		if minDate.IsZero() || tx.ParsedDate.Before(minDate) {
			minDate = tx.ParsedDate
		}
		if maxDate.IsZero() || tx.ParsedDate.After(maxDate) {
			maxDate = tx.ParsedDate
		}
	}

	if minDate.IsZero() || maxDate.IsZero() {
		return 30.0
	}

	days := maxDate.Sub(minDate).Hours() / 24.0
	if days < 1.0 {
		return 1.0
	}
	return days
}
