package analyzer

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"azaman-engine/internal/models"
)

var (
	// Counterparty name after "Transfer to/from", "Sent to", or NIP/DR/<acct>/<NAME>/
	counterpartyRe = regexp.MustCompile(`(?i)(?:transfer|trf|sent|received|payment)\s+(?:to|from)\s+([A-Za-z][A-Za-z ,.'-]{2,60})|NIP/(?:DR|CR)/[^/]+/([A-Za-z][A-Za-z ,.'-]{2,60})`)
	nonLetterRe    = regexp.MustCompile(`[^a-z ]+`)

	selfTransferKeywords = []string{"add money", "fund wallet", "wallet top up", "wallet topup", "own account", "to self", "self transfer", "my account", "between accounts", "inter-account", "intra account"}
	reversalKeywords     = []string{"reversal", "reversed", "rvsl", "rev/", "refund", "failed trf", "failed transfer"}
	failedKeywords       = []string{"failed", "unsuccessful", "pending", "declined"}
	familyKeywords       = []string{"mum", "dad", "mama", "papa", "mummy", "daddy", "sister", "brother", "family", "upkeep", "parents", "cousin", "aunty", "uncle", "wife", "husband", "iya", "baba", "black tax"}
)

// reversalWindow is how far after a failed debit we look for its reversal credit.
const reversalWindow = 7 * 24 * time.Hour

// cbnReversalDeadline is the CBN timeline for reversing failed electronic transfers.
const cbnReversalDeadline = 48 * time.Hour

// DetectSelfTransfers flags transfers between the account holder's own accounts and savings wallets.
// accountHolder may be empty, in which case only keyword and savings-rule detection applies.
func DetectSelfTransfers(txs []models.DecodedTransaction, accountHolder string) models.InternalTransfersReport {
	var report models.InternalTransfersReport
	holderTokens := nameTokens(accountHolder)

	for i := range txs {
		tx := &txs[i]
		if tx.Flags.IsBankFee || tx.Flags.IsReversal {
			continue
		}
		lower := strings.ToLower(tx.RawDescription + " " + tx.CleanMemo)
		if !tx.Flags.IsSavings {
			if containsAny(lower, selfTransferKeywords) || isSameName(holderTokens, counterparty(tx.RawDescription)) {
				tx.Flags.IsSelfTransfer = true
			}
		}
		if !tx.Flags.IsInternal() {
			continue
		}

		// Moving money to yourself is never a leak or a favor.
		tx.Flags.IsMidnightLeak = false
		tx.Flags.IsUrgent2k = false
		tx.Flags.IsIncome = false
		if tx.Flags.IsSelfTransfer {
			tx.Category = "Internal Transfers"
			tx.Classification = "Self Transfer"
		}
		report.Count++

		switch {
		case tx.Flags.IsSavings && tx.Debit > 0:
			report.SavingsDeposits += tx.Debit
		case tx.Flags.IsSavings && tx.Credit > 0:
			report.SavingsWithdrawal += tx.Credit
		case tx.Debit > 0:
			report.SelfTransfersOut += tx.Debit
		case tx.Credit > 0:
			report.SelfTransfersIn += tx.Credit
		}
	}

	report.NetSaved = round2(report.SavingsDeposits - report.SavingsWithdrawal)
	report.SavingsDeposits = round2(report.SavingsDeposits)
	report.SavingsWithdrawal = round2(report.SavingsWithdrawal)
	report.SelfTransfersOut = round2(report.SelfTransfersOut)
	report.SelfTransfersIn = round2(report.SelfTransfersIn)
	return report
}

// DetectReversals pairs failed debits with their reversal credits (same amount, credit memo mentions
// reversal/refund, within 7 days). Both legs are flagged so they drop out of spend, income, and duplicates.
// Debits whose memo says failed/pending but never got reversed are returned as dispute candidates.
func DetectReversals(txs []models.DecodedTransaction) models.ReversalsReport {
	report := models.ReversalsReport{
		Pairs:            []models.ReversalPair{},
		UnresolvedDebits: []models.DecodedTransaction{},
	}

	order := chronological(txs)
	used := make(map[int]bool)
	group := 1

	for _, ci := range order {
		credit := &txs[ci]
		if credit.Credit <= 0 || !containsAny(strings.ToLower(credit.RawDescription+" "+credit.CleanMemo), reversalKeywords) {
			continue
		}
		// Latest matching debit before the credit wins (closest in time).
		match := -1
		for _, di := range order {
			debit := &txs[di]
			if used[di] || debit.Debit <= 0 || math.Abs(debit.Debit-credit.Credit) > 0.01 {
				continue
			}
			if !debit.ParsedDate.IsZero() && !credit.ParsedDate.IsZero() {
				gap := credit.ParsedDate.Sub(debit.ParsedDate)
				if gap < 0 || gap > reversalWindow {
					continue
				}
			}
			match = di
		}
		if match < 0 {
			continue
		}
		debit := &txs[match]
		used[match] = true
		used[ci] = true

		id := fmt.Sprintf("REV-%03d", group)
		group++
		for _, t := range []*models.DecodedTransaction{debit, credit} {
			t.Flags.IsReversal = true
			t.Flags.IsMidnightLeak = false
			t.Flags.IsUrgent2k = false
			t.Flags.IsIncome = false
			t.ReversalGroup = id
		}
		credit.Category = "Reversals & Refunds"
		credit.Classification = "Reversal"

		var delay float64
		if !debit.ParsedDate.IsZero() && !credit.ParsedDate.IsZero() {
			delay = credit.ParsedDate.Sub(debit.ParsedDate).Hours()
		}
		late := delay > cbnReversalDeadline.Hours()
		if late {
			report.LateReversals++
		}
		report.ReversedCount++
		report.ReversedAmount += debit.Debit
		report.Pairs = append(report.Pairs, models.ReversalPair{
			GroupID:    id,
			Amount:     debit.Debit,
			DebitDate:  debit.Date,
			CreditDate: credit.Date,
			DelayHours: math.Round(delay*10) / 10,
			Late:       late,
		})
	}

	for i := range txs {
		tx := txs[i]
		if tx.Debit > 0 && !tx.Flags.IsReversal &&
			containsAny(strings.ToLower(tx.RawDescription+" "+tx.CleanMemo), failedKeywords) {
			report.UnresolvedCount++
			report.UnresolvedAmount += tx.Debit
			report.UnresolvedDebits = append(report.UnresolvedDebits, tx)
		}
	}

	report.ReversedAmount = round2(report.ReversedAmount)
	report.UnresolvedAmount = round2(report.UnresolvedAmount)
	return report
}

// DetectRecurring finds payments to the same recipient with a stable amount at a regular interval
// (weekly stipend, monthly black tax, ajo contributions, rent). These are planned obligations, so they
// are removed from the "Urgent 2k" impulse bucket and reported separately.
func DetectRecurring(txs []models.DecodedTransaction) []models.RecurringPayment {
	byRecipient := make(map[string][]int)
	for i, tx := range txs {
		if !tx.IsSpend() || tx.Flags.IsBankFee {
			continue
		}
		key := strings.ToLower(tx.Merchant)
		if cp := counterparty(tx.RawDescription); cp != "" {
			key = strings.Join(nameTokens(cp), " ")
		}
		if key == "" {
			continue
		}
		byRecipient[key] = append(byRecipient[key], i)
	}

	recurring := []models.RecurringPayment{}
	for recipient, idxs := range byRecipient {
		if len(idxs) < 3 {
			continue
		}
		sort.Slice(idxs, func(a, b int) bool { return txs[idxs[a]].ParsedDate.Before(txs[idxs[b]].ParsedDate) })
		if txs[idxs[0]].ParsedDate.IsZero() {
			continue
		}

		amounts := make([]float64, len(idxs))
		for k, i := range idxs {
			amounts[k] = txs[i].Debit
		}
		medAmt := median(amounts)
		stable := 0
		for _, a := range amounts {
			if math.Abs(a-medAmt) <= 0.1*medAmt {
				stable++
			}
		}
		if float64(stable) < 0.8*float64(len(amounts)) {
			continue
		}

		gaps := make([]float64, 0, len(idxs)-1)
		for k := 1; k < len(idxs); k++ {
			gaps = append(gaps, txs[idxs[k]].ParsedDate.Sub(txs[idxs[k-1]].ParsedDate).Hours()/24)
		}
		medGap := median(gaps)
		if medGap < 6 || coefficientOfVariation(gaps) > 0.35 {
			continue
		}

		lower := strings.ToLower(recipient + " " + txs[idxs[0]].RawDescription + " " + txs[idxs[0]].CleanMemo)
		kind := "Recurring Payment"
		category := "Recurring Obligations"
		switch {
		case txs[idxs[0]].Category == "Ajo & Thrift":
			kind, category = "Ajo / Thrift", "Ajo & Thrift"
		case txs[idxs[0]].Category == "Rent & Housing":
			kind, category = "Rent", "Rent & Housing"
		case containsAny(lower, familyKeywords) || txs[idxs[0]].Category == "Family & Favors":
			kind, category = "Black Tax (Family Support)", "Black Tax (Family Support)"
		}

		var total float64
		for _, i := range idxs {
			txs[i].Flags.IsRecurring = true
			txs[i].Flags.IsUrgent2k = false
			if txs[i].Category == "Family & Favors" || txs[i].Category == "" || txs[i].Category == "Personal Discretionary" {
				txs[i].Category = category
			}
			if txs[i].Classification == "Micro-Transfer (Urgent 2k)" {
				txs[i].Classification = "Planned Obligation"
			}
			total += txs[i].Debit
		}

		recurring = append(recurring, models.RecurringPayment{
			Recipient:     txs[idxs[0]].Merchant,
			Kind:          kind,
			AverageAmount: round2(total / float64(len(idxs))),
			IntervalDays:  math.Round(medGap*10) / 10,
			Count:         len(idxs),
			TotalAmount:   round2(total),
		})
	}

	sort.Slice(recurring, func(a, b int) bool { return recurring[a].TotalAmount > recurring[b].TotalAmount })
	return recurring
}

func counterparty(desc string) string {
	m := counterpartyRe.FindStringSubmatch(desc)
	if m == nil {
		return ""
	}
	name := m[1]
	if name == "" {
		name = m[2]
	}
	// Stop at bank/memo separators: "FAVOUR BALOGUN | OPay | 70..." or "SEGUN - Urgent 2k"
	if cut := strings.IndexAny(name, "|/"); cut >= 0 {
		name = name[:cut]
	}
	if cut := strings.Index(name, " - "); cut >= 0 {
		name = name[:cut]
	}
	return strings.TrimSpace(name)
}

func nameTokens(name string) []string {
	clean := nonLetterRe.ReplaceAllString(strings.ToLower(name), " ")
	return strings.Fields(clean)
}

// isSameName requires at least two shared name tokens and that the shorter name is fully contained
// in the longer one. A shared surname alone (siblings, parents) is not a self-transfer.
func isSameName(holder []string, other string) bool {
	otherTokens := nameTokens(other)
	if len(holder) < 2 || len(otherTokens) < 2 {
		return false
	}
	short, long := holder, otherTokens
	if len(short) > len(long) {
		short, long = long, short
	}
	set := make(map[string]bool, len(long))
	for _, t := range long {
		set[t] = true
	}
	for _, t := range short {
		if !set[t] {
			return false
		}
	}
	return true
}

func chronological(txs []models.DecodedTransaction) []int {
	order := make([]int, len(txs))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return txs[order[a]].ParsedDate.Before(txs[order[b]].ParsedDate) })
	return order
}

func containsAny(s string, keywords []string) bool {
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

func coefficientOfVariation(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	var mean float64
	for _, x := range v {
		mean += x
	}
	mean /= float64(len(v))
	if mean == 0 {
		return 0
	}
	var variance float64
	for _, x := range v {
		variance += (x - mean) * (x - mean)
	}
	return math.Sqrt(variance/float64(len(v))) / mean
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
