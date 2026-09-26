package analyzer

import (
	"sort"

	"azaman-engine/internal/models"
)

// AnalyzeMidnight identifies late-night discretionary spend (10 PM to 3 AM, or known late delivery/lounges).
func AnalyzeMidnight(txs []models.DecodedTransaction, totalDiscretionary float64) models.MidnightLeakReport {
	report := models.MidnightLeakReport{
		TopMerchants: []models.MerchantBurn{},
		Transactions: []models.DecodedTransaction{},
	}

	merchantMap := make(map[string]*models.MerchantBurn)

	for _, tx := range txs {
		// Only consider debits
		if tx.Amount >= 0 || tx.Debit <= 0 {
			continue
		}

		if tx.Flags.IsMidnightLeak {
			report.TotalAmount += tx.Debit
			report.Count++
			report.Transactions = append(report.Transactions, tx)

			merchantName := tx.Merchant
			if merchantName == "" {
				merchantName = "Other Late Night"
			}

			if mb, exists := merchantMap[merchantName]; exists {
				mb.TotalSpent += tx.Debit
				mb.Count++
			} else {
				merchantMap[merchantName] = &models.MerchantBurn{
					Merchant:   merchantName,
					TotalSpent: tx.Debit,
					Count:      1,
					Category:   tx.Category,
				}
			}
		}
	}

	if totalDiscretionary > 0 {
		report.PercentageOfDiscretionary = (report.TotalAmount / totalDiscretionary) * 100.0
	}

	// Sort top merchants by total spent descending
	for _, mb := range merchantMap {
		report.TopMerchants = append(report.TopMerchants, *mb)
	}
	sort.Slice(report.TopMerchants, func(i, j int) bool {
		return report.TopMerchants[i].TotalSpent > report.TopMerchants[j].TotalSpent
	})

	return report
}
