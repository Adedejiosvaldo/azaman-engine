package analyzer

import (
	"math"
	"sort"
	"time"

	"azaman-engine/internal/models"
)

// GenerateAudit orchestrates all anomaly detectors, leak aggregators, and cashflow calculators.
func GenerateAudit(
	rawTxs []models.DecodedTransaction,
	bankName string,
	sessionID string,
	fileFormat string,
	fileSize int64,
	startTime time.Time,
	accountHolder string,
) models.AuditReport {
	// 0. Nigerian-specific pre-passes on a private copy. Order matters: reversals cancel failed
	// debits first, then self/savings transfers drop out, then recurring payments leave Urgent 2k.
	txs := make([]models.DecodedTransaction, len(rawTxs))
	copy(txs, rawTxs)
	reversalsReport := DetectReversals(txs)
	internalReport := DetectSelfTransfers(txs, accountHolder)
	recurring := DetectRecurring(txs)

	// 1. Detect duplicates and get enriched transactions with duplicate flags
	duplicateReport, enrichedTxs := AnalyzeDuplicates(txs)

	// 2. Compute total discretionary outflow for midnight baseline
	var totalDiscretionary float64
	categoryTotals := make(map[string]*models.CategoryBreakdown)

	for _, tx := range enrichedTxs {
		if tx.IsSpend() {
			if tx.Classification == "Personal Discretionary" || tx.Category == "Late Night Food & Lounges" || tx.Category == "Food & Dining" || tx.Category == "Commute & Transit" {
				totalDiscretionary += tx.Debit
			}

			catName := tx.Category
			if catName == "" {
				catName = "Uncategorized"
			}

			if cat, exists := categoryTotals[catName]; exists {
				cat.Amount += tx.Debit
				cat.Count++
			} else {
				categoryTotals[catName] = &models.CategoryBreakdown{
					Category: catName,
					Amount:   tx.Debit,
					Count:    1,
				}
			}
		}
	}

	// 3. Period calculation
	periodDays := EstimatePeriodDays(enrichedTxs)

	// 4. Run detectors
	midnightReport := AnalyzeMidnight(enrichedTxs, totalDiscretionary)
	urgent2kReport := AnalyzeUrgent2k(enrichedTxs, periodDays)
	bankChargesReport := AnalyzeBankCharges(enrichedTxs)
	summary := CalculateRunway(enrichedTxs, periodDays)
	summary.AccountHolder = accountHolder
	insights := AnalyzeInsights(enrichedTxs, internalReport, recurring, summary)
	taxEstimate := EstimateTax(enrichedTxs, periodDays, summary.Currency)
	businessSplit := AnalyzeBusinessSplit(enrichedTxs)

	// 5. Total leak burn & percentage
	totalLeakBurn := midnightReport.TotalAmount + urgent2kReport.TotalAmount + duplicateReport.TotalPotentialRefund + bankChargesReport.TotalCharges
	var totalLeakPct float64
	if summary.TotalOutflow > 0 {
		totalLeakPct = (totalLeakBurn / summary.TotalOutflow) * 100.0
	}

	cashLeaks := models.CashLeaks{
		MidnightLeak:                 midnightReport,
		Urgent2k:                     urgent2kReport,
		DuplicateDebits:              duplicateReport,
		BankCharges:                  bankChargesReport,
		TotalLeakBurn:                math.Round(totalLeakBurn*100) / 100,
		TotalLeakPercentageOfOutflow: math.Round(totalLeakPct*100) / 100,
	}

	// 6. Category breakdown formatting
	var categories []models.CategoryBreakdown
	for _, cat := range categoryTotals {
		if summary.TotalOutflow > 0 {
			cat.Percentage = math.Round((cat.Amount/summary.TotalOutflow)*10000) / 100
		}
		cat.Amount = math.Round(cat.Amount*100) / 100
		categories = append(categories, *cat)
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Amount > categories[j].Amount
	})

	// 7. Audit metadata
	executionTimeMs := time.Since(startTime).Milliseconds()

	meta := models.AuditMeta{
		SessionID:       sessionID,
		BankDetected:    bankName,
		ExecutionTimeMs: executionTimeMs,
		ProcessedAt:     time.Now().UTC().Format(time.RFC3339),
		MemoryScrubbed:  true,
		FileFormat:      fileFormat,
		FileSize:        fileSize,
	}

	return models.AuditReport{
		Summary:      summary,
		CashLeaks:    cashLeaks,
		Categories:   categories,
		Reversals:    reversalsReport,
		Internal:     internalReport,
		Insights:     insights,
		Tax:          taxEstimate,
		Business:     businessSplit,
		Transactions: enrichedTxs,
		BankDetected: bankName,
		AuditMeta:    meta,
	}
}
