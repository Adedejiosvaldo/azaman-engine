package analyzer

import (
	"fmt"
	"math"
	"sort"
	"time"

	"azaman-engine/internal/models"
)

// Spending buckets, in display order. Family support sits in essentials on purpose: grading
// someone down for supporting relatives would misread how money works for most Nigerians.
var spendingBuckets = []struct{ key, label, description string }{
	{"builders", "Builders", "Savings, investments, business costs and education: spending that grows your future."},
	{"essentials", "Essentials", "Rent, food, transport, bills and family support: the cost of living."},
	{"lifestyle", "Lifestyle", "Shopping, subscriptions and personal treats: fine in moderation."},
	{"drains", "Drains", "Silent leaks, betting and late-night spending: money that buys little."},
	{"unclassified", "Unclassified", "Cash withdrawals and spending the engine could not place."},
}

var categoryBucket = map[string]string{
	"FinTech & Investments":       "builders",
	"Ajo & Thrift":                "builders",
	"Education":                   "builders",
	"Commercial & Business":       "builders",
	"Tech & Cloud Infrastructure": "builders",
	"Workspace & Co-working":      "builders",

	"Rent & Housing":             "essentials",
	"Airtime & Utilities":        "essentials",
	"Bills & Online Payments":    "essentials",
	"Commute & Transit":          "essentials",
	"Food & Dining":              "essentials",
	"Supermarket & Groceries":    "essentials",
	"Market & Small Vendors":     "essentials",
	"Black Tax (Family Support)": "essentials",
	"Family & Favors":            "essentials",
	"Recurring Obligations":      "essentials",
	"Religious Giving":           "essentials",
	"Loans & Debt Service":       "essentials",

	"Shopping & Retail":      "lifestyle",
	"Personal Care":          "lifestyle",
	"Subscriptions & TV":     "lifestyle",
	"Personal Discretionary": "lifestyle",

	"Late Night Food & Lounges": "drains",
	"Betting & Gaming":          "drains",
	"Bank Fees & Charges":       "drains",
}

// spendBucket places one spend transaction. Anything already counted in the leak total is a drain,
// so the drains bucket always contains every naira of total_leak_burn.
func spendBucket(tx models.DecodedTransaction, leakIDs map[string]float64) string {
	if tx.ID != "" {
		if _, ok := leakIDs[tx.ID]; ok {
			return "drains"
		}
	}
	if b, ok := categoryBucket[tx.Category]; ok {
		return b
	}
	if tx.Flags.IsBusiness || tx.Flags.IsTaxDeductible {
		return "builders"
	}
	return "unclassified"
}

// BuildSpendingMix splits real spend into buckets. Savings deposits are internal moves rather than
// spend, but putting money aside is the clearest builder there is, so they are added to builders.
func BuildSpendingMix(txs []models.DecodedTransaction, leakIDs map[string]float64, savingsDeposits float64) models.SpendingMix {
	type bucketAgg struct {
		amount float64
		count  int
		cats   map[string]*models.CategoryBreakdown
	}
	aggs := make(map[string]*bucketAgg, len(spendingBuckets))
	for _, b := range spendingBuckets {
		aggs[b.key] = &bucketAgg{cats: map[string]*models.CategoryBreakdown{}}
	}

	add := func(key, category string, amount float64, count int) {
		agg := aggs[key]
		agg.amount += amount
		agg.count += count
		if cat, ok := agg.cats[category]; ok {
			cat.Amount += amount
			cat.Count += count
		} else {
			agg.cats[category] = &models.CategoryBreakdown{Category: category, Amount: amount, Count: count}
		}
	}

	for _, tx := range txs {
		if !tx.IsSpend() {
			continue
		}
		category := tx.Category
		if category == "" {
			category = "Uncategorized"
		}
		add(spendBucket(tx, leakIDs), category, tx.Debit, 1)
	}
	if savingsDeposits > 0 {
		add("builders", "Savings deposits", savingsDeposits, 0)
	}

	var total float64
	for _, agg := range aggs {
		total += agg.amount
	}

	mix := models.SpendingMix{Total: round2(total)}
	for _, b := range spendingBuckets {
		agg := aggs[b.key]
		bucket := models.SpendingBucket{
			Key:           b.key,
			Label:         b.label,
			Description:   b.description,
			Amount:        round2(agg.amount),
			Count:         agg.count,
			TopCategories: []models.CategoryBreakdown{},
		}
		if total > 0 {
			bucket.Percentage = round2(agg.amount / total * 100)
		}
		for _, cat := range agg.cats {
			c := *cat
			c.Amount = round2(c.Amount)
			if total > 0 {
				c.Percentage = round2(cat.Amount / total * 100)
			}
			bucket.TopCategories = append(bucket.TopCategories, c)
		}
		sort.Slice(bucket.TopCategories, func(i, j int) bool {
			return bucket.TopCategories[i].Amount > bucket.TopCategories[j].Amount
		})
		if len(bucket.TopCategories) > 3 {
			bucket.TopCategories = bucket.TopCategories[:3]
		}
		mix.Buckets = append(mix.Buckets, bucket)
	}
	return mix
}

// BuildMonthly totals real inflow and outflow per calendar month. Months the statement only partly
// covers are marked so a half month is not read as a good or bad month.
func BuildMonthly(txs []models.DecodedTransaction) models.MonthlyReport {
	byMonth := make(map[string]*models.MonthlyFlow)
	var minDate, maxDate time.Time

	for _, tx := range txs {
		d := tx.ParsedDate
		if d.IsZero() {
			continue
		}
		if minDate.IsZero() || d.Before(minDate) {
			minDate = d
		}
		if maxDate.IsZero() || d.After(maxDate) {
			maxDate = d
		}

		key := d.Format("2006-01")
		m, ok := byMonth[key]
		if !ok {
			m = &models.MonthlyFlow{Month: key, Label: d.Format("Jan 2006")}
			byMonth[key] = m
		}
		switch {
		case tx.IsEarning():
			m.Inflow += tx.Credit
			m.Count++
		case tx.IsSpend():
			m.Outflow += tx.Debit
			m.Count++
		}
	}

	report := models.MonthlyReport{Months: []models.MonthlyFlow{}}
	if len(byMonth) == 0 {
		return report
	}

	keys := make([]string, 0, len(byMonth))
	for k := range byMonth {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	firstKey, lastKey := keys[0], keys[len(keys)-1]
	for _, k := range keys {
		m := byMonth[k]
		m.Net = round2(m.Inflow - m.Outflow)
		m.Inflow = round2(m.Inflow)
		m.Outflow = round2(m.Outflow)
		m.Partial = (k == firstKey && minDate.Day() > 3) ||
			(k == lastKey && maxDate.Day() <= daysInMonth(maxDate)-3)
		if m.Net > 0 {
			report.PositiveMonths++
		}
		report.Months = append(report.Months, *m)
	}

	for i := len(report.Months) - 1; i >= 0 && report.Months[i].Net > 0; i-- {
		report.PositiveStreak++
	}

	if len(report.Months) >= 2 {
		best, worst := report.Months[0], report.Months[0]
		for _, m := range report.Months[1:] {
			if m.Net > best.Net {
				best = m
			}
			if m.Net < worst.Net {
				worst = m
			}
		}
		report.BestMonth = best.Month
		report.WorstMonth = worst.Month
	}
	return report
}

func daysInMonth(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
}

// Grade bands, highest first.
var gradeBands = []struct {
	min      int
	grade    string
	headline string
}{
	{85, "A+", "Money master"},
	{75, "A", "Strong control of your money"},
	{67, "B+", "Solid, with room to tighten"},
	{58, "B", "Doing well, a few leaks to plug"},
	{50, "C+", "Small fixes add up fast"},
	{42, "C", "Money is slipping through"},
	{30, "D", "Room to grow"},
	{0, "F", "Time for a money reset"},
}

// Factor targets: the value at which a factor earns full points. Set to be reachable rather than
// ideal: the grade is a game people should be able to climb, not a lecture.
const (
	targetKeptPct    = 15.0
	targetSavingsPct = 5.0
	targetDrainsPct  = 10.0
	zeroDrainsPct    = 50.0 // drains at or above this share score nothing
	targetRunway     = 3.0  // months
	targetDebtPct    = 15.0
	zeroDebtPct      = 50.0
)

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// factorExplain says what each factor measures, for the info icon next to it.
var factorExplain = map[string]string{
	"money_kept":   "How much of the money that came in you still had at the end. Keeping 15% or more earns full points.",
	"saving_habit": "How much you moved into savings or investments (OWealth, PiggyVest, Cowrywise...). 5% of income earns full points.",
	"drains":       "Money lost to leaks: late-night spending, small urgent transfers, double charges, bank fees and betting. Under 10% of spending earns full points.",
	"runway":       "How many months your balance would last at your current spending. Three months earns full points.",
	"debt_load":    "How much of your income goes to loan repayments. Under 15% earns full points.",
}

// MarkUngraded replaces the letter grade when the statement could not be read reliably: a wrong
// grade is worse than none, because people share it.
func MarkUngraded(g *models.FinancialGrade, reason string) {
	g.Grade = "N/A"
	g.Score = 0
	g.Headline = "We couldn't grade this statement"
	g.NextGrade, g.PointsToNext, g.Focus, g.FocusTip = "", 0, "", ""
	g.LowConfidence = true
	g.Note = reason
}

// GradeAudit scores the statement out of 100 from five factors and maps it to a letter grade.
// Factors the statement has no data for (e.g. no income) are left out and the score is rescaled,
// so a spend-only account is not failed for lacking a salary line.
func GradeAudit(summary models.AuditSummary, insights models.NigeriaInsights, mix models.SpendingMix,
	periodDays float64, txCount int) models.FinancialGrade {
	sym := currencySymbol(summary.Currency)
	income := summary.TotalInflow - insights.BettingWinnings
	hasIncome := income > 0
	months := math.Max(periodDays/daysPerMonth, 1)
	perMonth := func(total float64) string { return sym + money(math.Max(total, 0)/months) }

	var factors []models.GradeFactor

	// 1. Money kept
	kept := models.GradeFactor{Key: "money_kept", Label: "Money kept", MaxPoints: 30, Applicable: hasIncome}
	if hasIncome {
		margin := summary.NetRetained / income * 100
		kept.Value = round2(margin)
		kept.Points = clamp01(margin/targetKeptPct) * kept.MaxPoints
		if margin >= 0 {
			kept.Detail = fmt.Sprintf("You kept %.1f%% of what came in.", margin)
		} else {
			kept.Detail = fmt.Sprintf("You spent %.1f%% more than came in.", -margin)
		}
		kept.Tip = fmt.Sprintf("Keep about %s more each month to hold on to %.0f%% of your income.",
			perMonth((targetKeptPct-margin)/100*income), targetKeptPct)
	} else {
		kept.Detail = "No income on this statement to compare spending against."
	}
	factors = append(factors, kept)

	// 2. Saving habit
	saving := models.GradeFactor{Key: "saving_habit", Label: "Saving habit", MaxPoints: 10, Applicable: hasIncome}
	if hasIncome {
		rate := insights.SavingsRatePct
		saving.Value = round2(rate)
		saving.Points = clamp01(rate/targetSavingsPct) * saving.MaxPoints
		saving.Detail = fmt.Sprintf("%.1f%% of income went into savings, investments or ajo.", rate)
		saving.Tip = fmt.Sprintf("Move %s a month into a savings wallet or ajo to reach %.0f%%.",
			perMonth((targetSavingsPct-rate)/100*income), targetSavingsPct)
	} else {
		saving.Detail = "No income on this statement to measure savings against."
	}
	factors = append(factors, saving)

	// 3. Drains
	drains := models.GradeFactor{Key: "drains", Label: "Drains", MaxPoints: 25, Applicable: mix.Total > 0}
	if mix.Total > 0 {
		var bucket models.SpendingBucket
		for _, b := range mix.Buckets {
			if b.Key == "drains" {
				bucket = b
			}
		}
		share := bucket.Percentage
		drains.Value = share
		drains.Points = clamp01((zeroDrainsPct-share)/(zeroDrainsPct-targetDrainsPct)) * drains.MaxPoints
		drains.Detail = fmt.Sprintf("%.1f%% of your spending went to drains.", share)
		drains.Tip = fmt.Sprintf("Cut drains by %s a month", perMonth((share-targetDrainsPct)/100*mix.Total))
		if len(bucket.TopCategories) > 0 {
			drains.Tip += fmt.Sprintf(", starting with %s", bucket.TopCategories[0].Category)
		}
		drains.Tip += "."
	} else {
		drains.Detail = "No spending on this statement."
	}
	factors = append(factors, drains)

	// 4. Runway
	burn := summary.WeightedMonthlyBurn
	runway := models.GradeFactor{Key: "runway", Label: "Runway", MaxPoints: 20, Applicable: burn > 0}
	if burn > 0 {
		r := summary.RunwayMonths
		runway.Value = r
		runway.Points = clamp01(r/targetRunway) * runway.MaxPoints
		switch {
		case r >= 999:
			runway.Detail = "Income covers spending; your balance is not being drawn down."
		case r < 1:
			runway.Detail = "Your balance covers less than a month of spending."
		default:
			runway.Detail = fmt.Sprintf("Your balance covers %.1f months of spending.", r)
		}
		runway.Tip = fmt.Sprintf("Build %s%s more buffer to reach %.0f months of runway.",
			sym, money(math.Max(targetRunway-r, 0)*burn), targetRunway)
	} else {
		runway.Detail = "No spending to measure runway against."
	}
	factors = append(factors, runway)

	// 5. Debt load
	debt := models.GradeFactor{Key: "debt_load", Label: "Debt load", MaxPoints: 15, Applicable: hasIncome}
	if hasIncome {
		ratio := insights.DebtServiceRatioPct
		debt.Value = ratio
		debt.Points = clamp01((zeroDebtPct-ratio)/(zeroDebtPct-targetDebtPct)) * debt.MaxPoints
		if ratio == 0 {
			debt.Detail = "No loan repayments found."
		} else {
			debt.Detail = fmt.Sprintf("Loan repayments take %.1f%% of income.", ratio)
		}
		debt.Tip = fmt.Sprintf("Bring loan repayments down by %s a month and avoid new loan apps.",
			perMonth((ratio-targetDebtPct)/100*income))
	} else {
		debt.Detail = "No income on this statement to compare repayments against."
	}
	factors = append(factors, debt)

	var earned, possible, bestGap float64
	grade := models.FinancialGrade{Factors: factors}
	for i := range factors {
		f := &grade.Factors[i]
		f.Explain = factorExplain[f.Key]
		f.Points = round1(f.Points)
		if !f.Applicable {
			f.Points = 0
			f.Tip = ""
			continue
		}
		earned += f.Points
		possible += f.MaxPoints
		gap := f.MaxPoints - f.Points
		if gap < 0.5 {
			f.Tip = ""
			continue
		}
		if gap > bestGap {
			bestGap = gap
			grade.Focus = f.Key
			grade.FocusTip = f.Tip
		}
	}

	if possible == 0 {
		grade.Grade = "N/A"
		grade.Headline = "Not enough activity to grade"
		grade.LowConfidence = true
		grade.Note = "This statement has no spending or income to score."
		return grade
	}

	grade.Score = int(math.Round(earned / possible * 100))
	for i, band := range gradeBands {
		if grade.Score >= band.min {
			grade.Grade = band.grade
			grade.Headline = band.headline
			if i > 0 {
				grade.NextGrade = gradeBands[i-1].grade
				grade.PointsToNext = float64(gradeBands[i-1].min - grade.Score)
			}
			break
		}
	}

	if periodDays < 25 || txCount < 15 {
		grade.LowConfidence = true
		grade.Note = "Based on less than a month of activity. Upload a longer statement for a fairer grade."
	}
	return grade
}
