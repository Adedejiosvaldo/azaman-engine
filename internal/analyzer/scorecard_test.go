package analyzer

import (
	"strings"
	"testing"
	"time"

	"azaman-engine/internal/models"
)

func spend(id, category string, amount float64, date time.Time) models.DecodedTransaction {
	return models.DecodedTransaction{ID: id, Category: category, Amount: -amount, Debit: amount, ParsedDate: date}
}

func earn(id string, amount float64, date time.Time) models.DecodedTransaction {
	return models.DecodedTransaction{ID: id, Category: "Income & Retainers", Amount: amount, Credit: amount,
		ParsedDate: date, Flags: models.TransactionFlags{IsIncome: true}}
}

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
}

func bucketByKey(t *testing.T, mix models.SpendingMix, key string) models.SpendingBucket {
	t.Helper()
	for _, b := range mix.Buckets {
		if b.Key == key {
			return b
		}
	}
	t.Fatalf("bucket %q missing", key)
	return models.SpendingBucket{}
}

func TestBuildSpendingMix(t *testing.T) {
	d := day(2026, 3, 10)
	txs := []models.DecodedTransaction{
		spend("rent", "Rent & Housing", 100000, d),
		spend("family", "Black Tax (Family Support)", 20000, d),
		spend("food", "Food & Dining", 30000, d), // in the leak set: must be a drain, not an essential
		spend("bet", "Betting & Gaming", 10000, d),
		spend("netflix", "Subscriptions & TV", 5000, d),
		spend("pos", "POS Cashout & Agency", 15000, d),
		earn("salary", 500000, d),
		{ID: "save", Amount: -40000, Debit: 40000, Flags: models.TransactionFlags{IsSavings: true}, ParsedDate: d},
	}
	leaks := map[string]float64{"food": 30000}

	mix := BuildSpendingMix(txs, leaks, 40000)

	if mix.Total != 220000 {
		t.Fatalf("total = %v, want 220000 (180k spend + 40k savings)", mix.Total)
	}
	cases := map[string]float64{
		"builders":     40000,
		"essentials":   120000,
		"lifestyle":    5000,
		"drains":       40000,
		"unclassified": 15000,
	}
	for key, want := range cases {
		if got := bucketByKey(t, mix, key).Amount; got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}
	if len(mix.Buckets) != 5 || mix.Buckets[0].Key != "builders" {
		t.Errorf("buckets out of order: %+v", mix.Buckets)
	}
}

func TestBuildMonthly(t *testing.T) {
	txs := []models.DecodedTransaction{
		earn("i1", 300000, day(2026, 1, 15)),
		spend("s1", "Food & Dining", 350000, day(2026, 1, 20)),
		earn("i2", 300000, day(2026, 2, 1)),
		spend("s2", "Food & Dining", 100000, day(2026, 2, 10)),
		earn("i3", 300000, day(2026, 3, 2)),
		spend("s3", "Food & Dining", 50000, day(2026, 3, 31)),
		{ID: "self", Amount: -90000, Debit: 90000, ParsedDate: day(2026, 2, 5),
			Flags: models.TransactionFlags{IsSelfTransfer: true}},
	}

	r := BuildMonthly(txs)

	if len(r.Months) != 3 {
		t.Fatalf("months = %d, want 3", len(r.Months))
	}
	if !r.Months[0].Partial || r.Months[2].Partial {
		t.Errorf("partial flags = %v/%v, want first partial, last full", r.Months[0].Partial, r.Months[2].Partial)
	}
	if feb := r.Months[1]; feb.Outflow != 100000 || feb.Net != 200000 {
		t.Errorf("Feb outflow/net = %v/%v; self-transfers must be excluded", feb.Outflow, feb.Net)
	}
	if r.PositiveMonths != 2 || r.PositiveStreak != 2 {
		t.Errorf("positive months/streak = %d/%d, want 2/2", r.PositiveMonths, r.PositiveStreak)
	}
	if r.BestMonth != "2026-03" || r.WorstMonth != "2026-01" {
		t.Errorf("best/worst = %s/%s", r.BestMonth, r.WorstMonth)
	}
	if r.Months[0].Label != "Jan 2026" {
		t.Errorf("label = %q", r.Months[0].Label)
	}
}

func mixWithDrains(total, drainsPct float64) models.SpendingMix {
	return models.SpendingMix{Total: total, Buckets: []models.SpendingBucket{
		{Key: "drains", Percentage: drainsPct, TopCategories: []models.CategoryBreakdown{{Category: "Betting & Gaming"}}},
	}}
}

func TestGradeAuditStrongAndWeak(t *testing.T) {
	strong := GradeAudit(
		models.AuditSummary{TotalInflow: 1000000, NetRetained: 400000, RunwayMonths: 8, WeightedMonthlyBurn: 200000},
		models.NigeriaInsights{SavingsRatePct: 15},
		mixWithDrains(600000, 3), 90, 120)
	if strong.Grade != "A+" || strong.Score != 100 {
		t.Errorf("strong = %s (%d), want A+ (100)", strong.Grade, strong.Score)
	}
	if strong.NextGrade != "" || strong.Focus != "" {
		t.Errorf("A+ should have no next grade or focus, got %q/%q", strong.NextGrade, strong.Focus)
	}

	weak := GradeAudit(
		models.AuditSummary{TotalInflow: 500000, NetRetained: -100000, RunwayMonths: 0.3, WeightedMonthlyBurn: 300000},
		models.NigeriaInsights{SavingsRatePct: 0, DebtServiceRatioPct: 45},
		mixWithDrains(600000, 55), 90, 120)
	if weak.Grade != "F" {
		t.Errorf("weak = %s (%d), want F", weak.Grade, weak.Score)
	}
	if weak.NextGrade != "D" || weak.PointsToNext <= 0 {
		t.Errorf("weak next = %s (+%v)", weak.NextGrade, weak.PointsToNext)
	}
	if weak.Focus != "money_kept" || !strings.Contains(weak.FocusTip, "₦") {
		t.Errorf("focus = %q tip = %q", weak.Focus, weak.FocusTip)
	}
}

func TestGradeAuditWithoutIncomeRescales(t *testing.T) {
	g := GradeAudit(
		models.AuditSummary{TotalInflow: 0, NetRetained: -200000, RunwayMonths: 6, WeightedMonthlyBurn: 200000},
		models.NigeriaInsights{},
		mixWithDrains(200000, 5), 60, 40)

	for _, f := range g.Factors {
		incomeBased := f.Key == "money_kept" || f.Key == "saving_habit" || f.Key == "debt_load"
		if incomeBased == f.Applicable {
			t.Errorf("%s applicable = %v", f.Key, f.Applicable)
		}
	}
	// Drains and runway are both perfect, so the rescaled score is 100 rather than 45.
	if g.Score != 100 {
		t.Errorf("score = %d, want 100 after rescaling", g.Score)
	}
}

func TestGradeAuditLowConfidenceAndEmpty(t *testing.T) {
	short := GradeAudit(
		models.AuditSummary{TotalInflow: 100000, NetRetained: 20000, RunwayMonths: 2, WeightedMonthlyBurn: 80000},
		models.NigeriaInsights{SavingsRatePct: 5}, mixWithDrains(80000, 10), 10, 8)
	if !short.LowConfidence || short.Note == "" {
		t.Errorf("short statement should be low confidence with a note")
	}

	empty := GradeAudit(models.AuditSummary{}, models.NigeriaInsights{}, models.SpendingMix{}, 30, 0)
	if empty.Grade != "N/A" || !empty.LowConfidence {
		t.Errorf("empty = %s, want N/A low confidence", empty.Grade)
	}
}
