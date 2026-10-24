package analyzer

import (
	"fmt"
	"sort"

	"azaman-engine/internal/models"
)

// minFinding is the smallest amount worth calling out; below it a finding is noise.
const minFinding = 1000.0

// BuildFindings turns the audit's totals into a short list of problems, each with what it cost and
// the first thing to do about it, biggest first. The wording describes habits, not advice.
func BuildFindings(currency string, leaks models.CashLeaks, rev models.ReversalsReport, in models.NigeriaInsights) []models.Finding {
	sym := currencySymbol(currency)
	var out []models.Finding
	add := func(key, title string, amount float64, detail, action string) {
		if amount >= minFinding {
			out = append(out, models.Finding{Key: key, Title: title, Amount: round2(amount), Detail: detail, Action: action})
		}
	}

	u := leaks.Urgent2k
	add("urgent_2k", "Small transfers are quietly adding up", u.TotalAmount,
		fmt.Sprintf("%d transfers of %s1k–15k, about %.1f a week. Each feels small; together they're one of your biggest costs.", u.Count, sym, u.FrequencyPerWeek),
		"Pick a weekly amount for favours and send from that pot only. When it's empty, the answer is next week.")

	m := leaks.MidnightLeak
	add("midnight", "Late nights are costing you", m.TotalAmount,
		fmt.Sprintf("%d payments between 10pm and 4am: food, rides and lounges.", m.Count),
		"Order before 9pm, or set a late-night limit in your bank app.")

	d := leaks.DuplicateDebits
	add("duplicates", "You may have been charged twice", d.TotalPotentialRefund,
		fmt.Sprintf("%d payments repeat the same amount to the same place within minutes.", d.Count),
		"Check each pair in the ledger. If you didn't make both, ask your bank to reverse the extra one; they must act within 48 hours.")

	add("unreversed", "Failed transfers that never came back", rev.UnresolvedAmount,
		fmt.Sprintf("%d debits marked failed or pending with no refund in sight.", rev.UnresolvedCount),
		"Raise a complaint with your bank, quoting the date and amount. The CBN rule is a refund within 48 hours.")

	add("betting", "Betting is taking a real share", in.BettingSpend-in.BettingWinnings,
		fmt.Sprintf("%s staked across %d bets, %s won back.", sym+money(in.BettingSpend), in.BettingCount, sym+money(in.BettingWinnings)),
		"Set a monthly betting budget and keep it in a separate wallet.")

	add("bank_charges", "Bank charges you can cut", leaks.BankCharges.TotalCharges,
		fmt.Sprintf("%d charges: transfer fees, SMS alerts, USSD, stamp duty.", leaks.BankCharges.Count),
		"Use app transfers instead of USSD, and switch SMS alerts to free push notifications.")

	sort.SliceStable(out, func(i, j int) bool { return out[i].Amount > out[j].Amount })
	if out == nil {
		out = []models.Finding{}
	}
	return out
}
