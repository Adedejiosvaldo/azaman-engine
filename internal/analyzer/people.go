package analyzer

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"azaman-engine/internal/models"
	"azaman-engine/internal/shazam"
)

const (
	familyThreshold = 1.5  // score at or above: family/friend
	vendorThreshold = -1.5 // score at or below: vendor
	maxReview       = 10
)

type personStats struct {
	name          string
	tokens        []string
	debitIdx      []int
	totalSent     float64
	count         int
	round, odd    int
	vendorBank    int
	purchaseMemos int
	supportMemos  int
	receivedFrom  bool
}

// ScorePeople decides, per counterparty, whether transfers without a typed purpose are family/friend
// support or payments to a vendor, from signals about the person rather than any single payment:
//
//	shares the holder's surname   +2.5   family rarely sells to you... but can (hence a score, not a rule)
//	money also comes back         +1.5   two-way flow is typical of family and friends
//	mostly round amounts          +1.0   gifts and allowances are round (₦2,000, ₦5,000)
//	mostly odd amounts            -1.5   prices are odd (₦1,050, ₦3,810)
//	Moniepoint account            -2.0   mostly traders' business accounts and POS terminals
//	purpose memos                 ±1 each (capped ±3)
//
// Rows decided by the payment's own memo or by a user label are never changed. People the engine
// is unsure about are returned, ranked by money sent, for the user to confirm.
func ScorePeople(txs []models.DecodedTransaction, accountHolder string) []models.PersonReview {
	holder := map[string]bool{}
	for _, t := range nameTokens(accountHolder) {
		holder[t] = true
	}

	people := map[string]*personStats{}
	key := func(name string) string { return strings.Join(sortedTokens(name), " ") }

	for i, tx := range txs {
		if tx.Counterparty == "" || tx.Flags.IsInternal() || tx.Flags.IsBankFee {
			continue
		}
		k := key(tx.Counterparty)
		if k == "" || isOrganisation(tx.Counterparty) {
			continue
		}
		p := people[k]
		if p == nil {
			p = &personStats{name: titleWords(tx.Counterparty), tokens: nameTokens(tx.Counterparty)}
			people[k] = p
		}
		if tx.Credit > 0 {
			p.receivedFrom = true
			// Money from a relative is support, not earnings (it stays in inflow, but not in
			// taxable income or the business split). Only name-guessed rows are relabelled.
			if shared := sharedTokens(p.tokens, holder); len(shared) > 0 && len(shared) < len(p.tokens) &&
				tx.DecisionBasis == "name" && tx.Flags.IsIncome {
				txs[i].Category = "Family & Favors"
				txs[i].Classification = "Family Support Received"
				txs[i].Merchant = "Transfer from " + p.name
				txs[i].Flags.IsIncome = false
				txs[i].Flags.IsBusiness = false
				txs[i].DecisionBasis = "person"
			}
			continue
		}
		if tx.Debit <= 0 {
			continue
		}
		p.totalSent += tx.Debit
		p.count++
		// Below ₦1,000 amounts are ambiguous (₦350 is a normal gift and a normal price)
		if tx.Debit >= 1000 {
			if math.Mod(tx.Debit, 500) == 0 {
				p.round++
			} else {
				p.odd++
			}
		}
		if strings.Contains(strings.ToLower(tx.RawDescription), "monie point") || strings.Contains(strings.ToLower(tx.RawDescription), "moniepoint") {
			p.vendorBank++
		}
		switch {
		case tx.DecisionBasis == "memo" && tx.Category == "Market & Small Vendors":
			p.purchaseMemos++
		case tx.DecisionBasis == "memo" && tx.Category == "Family & Favors":
			p.supportMemos++
		}
		// Only rows decided by the name heuristics (family by default, or the Moniepoint vendor
		// guess) may be re-decided here.
		if tx.DecisionBasis == "name" && tx.Confidence < 1 &&
			(tx.Category == "Family & Favors" || tx.Category == "Market & Small Vendors") {
			p.debitIdx = append(p.debitIdx, i)
		}
	}

	var review []models.PersonReview
	for _, p := range people {
		sent := p.count
		if sent == 0 {
			continue
		}
		sized := p.round + p.odd // payments of ₦1,000+ whose roundness is meaningful
		score, reasons := 0.0, []string{}
		if shared := sharedTokens(p.tokens, holder); len(shared) > 0 && len(shared) < len(p.tokens) {
			score += 2.5
			reasons = append(reasons, "shares your name ("+strings.Title(strings.Join(shared, ", "))+")")
		}
		if p.receivedFrom {
			score += 1.5
			reasons = append(reasons, "money also comes back from them")
		}
		switch {
		case sized > 0 && float64(p.round)/float64(sized) >= 0.7:
			score += 1
			reasons = append(reasons, "round amounts (gifts, allowances)")
		case sized >= 2 && float64(p.odd)/float64(sized) >= 0.5:
			score -= 1.5
			reasons = append(reasons, "odd amounts like prices")
		}
		if p.vendorBank*2 > sent {
			score -= 2
			reasons = append(reasons, "Moniepoint account (mostly traders)")
		}
		if p.purchaseMemos > 0 {
			score -= math.Min(float64(p.purchaseMemos), 3)
			reasons = append(reasons, fmt.Sprintf("%d payment memo(s) look like purchases", p.purchaseMemos))
		}
		if p.supportMemos > 0 {
			score += math.Min(float64(p.supportMemos), 3)
			reasons = append(reasons, fmt.Sprintf("%d payment memo(s) look like support", p.supportMemos))
		}

		guess := "unsure"
		switch {
		case score >= familyThreshold:
			guess = "family"
		case score <= vendorThreshold:
			guess = "vendor"
		}

		for _, i := range p.debitIdx {
			tx := &txs[i]
			switch guess {
			case "vendor":
				tx.Merchant = p.name + " (Vendor)"
				tx.Category = "Market & Small Vendors"
				tx.Classification = "Personal Purchase"
				tx.Flags.IsUrgent2k = false
				tx.Confidence = 0.8
				tx.DecisionBasis = "person"
			case "family":
				tx.Merchant = "Transfer to " + p.name
				tx.Category = "Family & Favors"
				tx.Flags.IsUrgent2k = tx.Debit >= 1000 && tx.Debit <= 15000 && (tx.Currency == "NGN" || tx.Currency == "")
				tx.Classification = "Personal Transfer"
				if tx.Flags.IsUrgent2k {
					tx.Classification = "Micro-Transfer (Urgent 2k)"
				}
				tx.Confidence = 0.85
				tx.DecisionBasis = "person"
			}
		}

		// Ask the user about people the score can't settle, and weak guesses on undecided rows
		if len(p.debitIdx) > 0 && (guess == "unsure" || math.Abs(score) < 2.5) {
			if len(reasons) == 0 {
				reasons = append(reasons, "no typed purpose and no strong signal")
			}
			review = append(review, models.PersonReview{
				Name:      p.name,
				Count:     sent,
				TotalSent: round2(p.totalSent),
				Guess:     guess,
				Score:     math.Round(score*10) / 10,
				Reasons:   reasons,
			})
		}
	}

	sort.Slice(review, func(a, b int) bool { return review[a].TotalSent > review[b].TotalSent })
	if len(review) > maxReview {
		review = review[:maxReview]
	}
	if review == nil {
		review = []models.PersonReview{}
	}
	return review
}

// orgWords mark a business or institution rather than a person.
var orgWords = []string{"ltd", "limited", "plc", "offer", "pay", "payments", "network", "mfb", "bank",
	"microfinance", "store", "stores", "supermarket", "ventures", "enterprise", "enterprises", "services",
	"global", "nig", "company", "investment", "investments", "capital", "wallet", "foundation", "church"}

// isOrganisation reports whether a counterparty is a business or institution. Those are never
// family, so they are left out of person scoring and the "who's who" review.
func isOrganisation(name string) bool {
	if shazam.FindNameRule(name) != nil {
		return true
	}
	lower := strings.ToLower(name)
	for _, w := range orgWords {
		if shazam.ContainsWord(lower, w) {
			return true
		}
	}
	return false
}

func sortedTokens(name string) []string {
	t := nameTokens(name)
	sort.Strings(t)
	return t
}

// sharedTokens returns the counterparty's name tokens that also appear in the holder's name
// (at least 3 letters, so initials don't count).
func sharedTokens(tokens []string, holder map[string]bool) []string {
	var shared []string
	for _, t := range tokens {
		if len(t) >= 3 && holder[t] {
			shared = append(shared, t)
		}
	}
	return shared
}
