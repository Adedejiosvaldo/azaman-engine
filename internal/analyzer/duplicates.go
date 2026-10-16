package analyzer

import (
	"fmt"
	"math"
	"sort"
	"time"

	"azaman-engine/internal/models"
)

// duplicateWindow is how close two identical timed debits must be to look like a double charge.
const duplicateWindow = 10 * time.Minute

// hasTimeOfDay is false for date-only rows, which parse to exactly 00:00:00.
func hasTimeOfDay(t time.Time) bool {
	return !t.IsZero() && (t.Hour() != 0 || t.Minute() != 0 || t.Second() != 0)
}

// AnalyzeDuplicates identifies potential duplicate charges (identical amount & vendor within short window).
func AnalyzeDuplicates(txs []models.DecodedTransaction) (models.DuplicateDebitsReport, []models.DecodedTransaction) {
	report := models.DuplicateDebitsReport{
		Pairs: []models.DuplicatePair{},
	}

	// Create a copy to allow modifying flags
	enrichedTxs := make([]models.DecodedTransaction, len(txs))
	copy(enrichedTxs, txs)

	// Filter debit transactions
	type debitRef struct {
		index int
		tx    *models.DecodedTransaction
	}

	var debits []debitRef
	for i := range enrichedTxs {
		tx := &enrichedTxs[i]
		if tx.IsSpend() && !tx.Flags.IsBankFee && !tx.Flags.IsRecurring {
			debits = append(debits, debitRef{index: i, tx: tx})
		}
	}

	// Sort debits by ParsedDate if available
	sort.Slice(debits, func(i, j int) bool {
		return debits[i].tx.ParsedDate.Before(debits[j].tx.ParsedDate)
	})

	matchedIndices := make(map[int]bool)
	groupIDCounter := 1

	for i := 0; i < len(debits); i++ {
		if matchedIndices[i] {
			continue
		}

		itemA := debits[i]
		var duplicateCluster []debitRef
		duplicateCluster = append(duplicateCluster, itemA)

		for j := i + 1; j < len(debits); j++ {
			if matchedIndices[j] {
				continue
			}

			itemB := debits[j]

			// Must match amount exactly
			if math.Abs(itemA.tx.Debit-itemB.tx.Debit) > 0.01 {
				continue
			}

			// Must match vendor or clean memo
			if itemA.tx.Merchant != itemB.tx.Merchant && itemA.tx.CleanMemo != itemB.tx.CleanMemo {
				continue
			}

			// A double charge lands within minutes. With times of day, repeat purchases hours apart
			// (the same data plan every morning) are not duplicates; date-only rows need the same day.
			isTimeClose := false
			if hasTimeOfDay(itemA.tx.ParsedDate) && hasTimeOfDay(itemB.tx.ParsedDate) {
				diff := itemB.tx.ParsedDate.Sub(itemA.tx.ParsedDate)
				if diff < 0 {
					diff = -diff
				}
				isTimeClose = diff <= duplicateWindow
			} else if !itemA.tx.ParsedDate.IsZero() && !itemB.tx.ParsedDate.IsZero() {
				isTimeClose = itemA.tx.ParsedDate.Format("2006-01-02") == itemB.tx.ParsedDate.Format("2006-01-02")
			} else {
				isTimeClose = itemA.tx.Date == itemB.tx.Date && itemA.tx.Date != ""
			}

			if isTimeClose {
				duplicateCluster = append(duplicateCluster, itemB)
				matchedIndices[j] = true
			}
		}

		if len(duplicateCluster) > 1 {
			matchedIndices[i] = true
			groupID := fmt.Sprintf("DUP-%03d", groupIDCounter)
			groupIDCounter++

			var clusterTxs []models.DecodedTransaction
			firstDate := itemA.tx.Date

			// Mark each transaction in the cluster
			for cIdx, cItem := range duplicateCluster {
				enrichedTxs[cItem.index].Flags.IsDuplicateDebit = true
				enrichedTxs[cItem.index].DuplicateGroup = groupID
				clusterTxs = append(clusterTxs, enrichedTxs[cItem.index])

				// If not the first original transaction, count towards refund
				if cIdx > 0 {
					report.TotalPotentialRefund += cItem.tx.Debit
					report.Count++
				}
			}

			var diffMins int64
			if !itemA.tx.ParsedDate.IsZero() && !duplicateCluster[1].tx.ParsedDate.IsZero() {
				diff := duplicateCluster[1].tx.ParsedDate.Sub(itemA.tx.ParsedDate)
				diffMins = int64(diff.Minutes())
			}

			report.Pairs = append(report.Pairs, models.DuplicatePair{
				Merchant:        itemA.tx.Merchant,
				Amount:          itemA.tx.Debit,
				DifferenceMins:  diffMins,
				FirstOccurredAt: firstDate,
				Transactions:    clusterTxs,
			})
		}
	}

	return report, enrichedTxs
}
