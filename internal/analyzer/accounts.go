package analyzer

import (
	"math"
	"time"

	"azaman-engine/internal/models"
)

// crossAccountWindow is how far apart the two legs of a transfer between the holder's own accounts
// can be. Interbank transfers usually land within minutes, but date-only rows and weekend
// settlement can push the credit to the next day or two.
const (
	crossAccountBefore = 24 * time.Hour // a date-only credit (00:00) can sort before a timed debit the same day
	crossAccountAfter  = 72 * time.Hour
)

// MatchCrossAccountTransfers pairs a debit in one uploaded statement with a credit of the same
// amount in another, close in time, and flags both legs as self-transfers. Without this, moving
// ₦50k from GTBank to OPay counts as ₦50k spent and ₦50k earned.
//
// When the holder's name is known, a leg that names a different person is never matched: a ₦5,000
// payment out of OPay and a friend's ₦5,000 into Kuda on the same day are not a self-transfer.
// Returns the number of pairs matched.
func MatchCrossAccountTransfers(txs []models.DecodedTransaction, accountHolder string) int {
	accounts := make(map[string]bool)
	for _, tx := range txs {
		accounts[tx.Account] = true
	}
	if len(accounts) < 2 {
		return 0
	}

	holder := nameTokens(accountHolder)
	eligible := func(tx *models.DecodedTransaction) bool {
		if tx.Flags.IsBankFee || tx.Flags.IsReversal || tx.Flags.IsSavings || tx.Flags.IsLoan {
			return false
		}
		if len(holder) >= 2 {
			if name := counterparty(tx.RawDescription); name != "" && !isSameName(holder, name) {
				return false
			}
		}
		return true
	}

	order := chronological(txs)
	used := make(map[int]bool)
	pairs := 0

	for _, di := range order {
		debit := &txs[di]
		if used[di] || debit.Debit <= 0 || !eligible(debit) {
			continue
		}
		best, bestGap := -1, time.Duration(math.MaxInt64)
		for _, ci := range order {
			credit := &txs[ci]
			if used[ci] || credit.Credit <= 0 || credit.Account == debit.Account ||
				math.Abs(credit.Credit-debit.Debit) > 0.01 || !eligible(credit) {
				continue
			}
			gap := time.Duration(0)
			if !debit.ParsedDate.IsZero() && !credit.ParsedDate.IsZero() {
				gap = credit.ParsedDate.Sub(debit.ParsedDate)
				if gap < -crossAccountBefore || gap > crossAccountAfter {
					continue
				}
			}
			if gap < 0 {
				gap = -gap
			}
			if gap < bestGap {
				best, bestGap = ci, gap
			}
		}
		if best < 0 {
			continue
		}
		used[di], used[best] = true, true
		debit.Flags.IsSelfTransfer = true
		txs[best].Flags.IsSelfTransfer = true
		pairs++
	}
	return pairs
}

// FillAccountTotals adds each account's own inflow, outflow, closing balance and date range,
// computed from the audited transactions (after internal moves were removed).
func FillAccountTotals(accounts []models.AccountSummary, txs []models.DecodedTransaction) {
	index := make(map[string]int, len(accounts))
	for i, a := range accounts {
		index[a.Label] = i
	}
	type span struct{ first, last, balanceAt time.Time }
	spans := make([]span, len(accounts))

	for _, tx := range txs {
		i, ok := index[tx.Account]
		if !ok {
			continue
		}
		a := &accounts[i]
		s := &spans[i]
		if tx.Currency != "" {
			a.Currency = tx.Currency
		}
		if tx.IsEarning() {
			a.Inflow += tx.Credit
		} else if tx.IsSpend() {
			a.Outflow += tx.Debit
		}
		if !tx.ParsedDate.IsZero() {
			if s.first.IsZero() || tx.ParsedDate.Before(s.first) {
				s.first = tx.ParsedDate
			}
			if tx.ParsedDate.After(s.last) {
				s.last = tx.ParsedDate
			}
		}
		if tx.Balance != 0 && !tx.ParsedDate.Before(s.balanceAt) {
			a.ClosingBalance = tx.Balance
			s.balanceAt = tx.ParsedDate
		}
	}

	for i := range accounts {
		a := &accounts[i]
		a.Inflow = round2(a.Inflow)
		a.Outflow = round2(a.Outflow)
		a.ClosingBalance = round2(a.ClosingBalance)
		if a.Currency == "" {
			a.Currency = "NGN"
		}
		if !spans[i].first.IsZero() {
			a.PeriodStart = spans[i].first.Format("2006-01-02")
			a.PeriodEnd = spans[i].last.Format("2006-01-02")
		}
	}
}
