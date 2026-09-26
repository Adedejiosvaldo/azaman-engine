package analyzer

import (
	"fmt"
	"math"
	"sort"
	"time"

	"azaman-engine/internal/models"
)

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
		if tx.Amount < 0 && tx.Debit > 0 && !tx.Flags.IsBankFee {
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

			// Must be within 24 hours or same calendar day
			isTimeClose := false
			if !itemA.tx.ParsedDate.IsZero() && !itemB.tx.ParsedDate.IsZero() {
				diff := itemB.tx.ParsedDate.Sub(itemA.tx.ParsedDate)
				if diff < 0 {
					diff = -diff
				}
				if diff <= 24*time.Hour {
					isTimeClose = true
				}
			} else if itemA.tx.Date == itemB.tx.Date && itemA.tx.Date != "" {
				isTimeClose = true
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
