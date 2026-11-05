package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"azaman-engine/internal/models"
)

func levyTx(id, desc string, amount float64, when time.Time) models.DecodedTransaction {
	tx := debitTx(id, "GTBank", desc, amount, when)
	tx.Category = "Bank Fees & Charges"
	tx.Flags.IsBankFee = true
	return tx
}

func TestLeviesDoNotCostGradePoints(t *testing.T) {
	txs := []models.DecodedTransaction{
		creditTx("s", "GTBank", "Salary ACME LTD", 300000, at(1, 9)),
		debitTx("r", "GTBank", "Rent payment landlord", 100000, at(2, 9)),
		levyTx("l1", "Stamp Duty Charge", 50, at(1, 9)),
		levyTx("l2", "EMTL Electronic Money Transfer Levy", 50, at(1, 9)),
	}
	report := GenerateAudit(txs, "GTBank", "s1", "csv", 0, time.Now(), "")
	if report.SpendingMix.Levies != 100 {
		t.Fatalf("levies = %v, want 100", report.SpendingMix.Levies)
	}
	if report.CashLeaks.TotalLeakBurn < 100 {
		t.Errorf("levies should still be in the leak total, got %v", report.CashLeaks.TotalLeakBurn)
	}
	for _, f := range report.Grade.Factors {
		if f.Key == "drains" && f.Value != 0 {
			t.Errorf("drains share should leave levies out, got %v", f.Value)
		}
	}
}

func TestMixedCurrenciesGradeMainCurrencyOnly(t *testing.T) {
	usd := func(tx models.DecodedTransaction) models.DecodedTransaction {
		tx.Currency = "USD"
		tx.Account = "Geegpay"
		return tx
	}
	txs := []models.DecodedTransaction{
		creditTx("s", "GTBank", "Salary ACME LTD", 300000, at(1, 9)),
		debitTx("a", "GTBank", "Rent payment landlord", 100000, at(2, 9)),
		debitTx("b", "GTBank", "Shoprite groceries", 20000, at(5, 9)),
		usd(creditTx("u", "", "Client payment", 2000, at(3, 9))),
	}
	report := GenerateAudit(txs, "GTBank + Geegpay", "s1", "csv", 0, time.Now(), "")
	if report.Grade.Currency != "NGN" {
		t.Errorf("grade currency = %q, want NGN", report.Grade.Currency)
	}
	if !strings.Contains(report.Grade.Note, "NGN accounts only") {
		t.Errorf("note should say only NGN was graded, got %q", report.Grade.Note)
	}
}

func TestMonthsGetTheirOwnGrade(t *testing.T) {
	month := func(m time.Month, day int) time.Time { return time.Date(2026, m, day, 9, 0, 0, 0, time.UTC) }
	var txs []models.DecodedTransaction
	for i, m := range []time.Month{time.January, time.February} {
		id := string(rune('a' + i))
		txs = append(txs,
			creditTx(id+"s", "GTBank", "Salary ACME LTD", 300000, month(m, 1)),
			debitTx(id+"r", "GTBank", "Rent payment landlord", 100000, month(m, 2)),
			debitTx(id+"f", "GTBank", "Shoprite groceries", 30000, month(m, 28)),
		)
	}
	report := GenerateAudit(txs, "GTBank", "s1", "csv", 0, time.Now(), "")
	if len(report.Monthly.Months) != 2 {
		t.Fatalf("months = %d, want 2", len(report.Monthly.Months))
	}
	for _, m := range report.Monthly.Months {
		if m.Grade == "" {
			t.Errorf("%s has no grade", m.Label)
		}
	}
}

func TestFocusCarriesItsAmount(t *testing.T) {
	g := GradeAudit(
		models.AuditSummary{TotalInflow: 500000, NetRetained: -100000, RunwayMonths: 0.3, WeightedMonthlyBurn: 300000},
		models.NigeriaInsights{DebtServiceRatioPct: 45},
		mixWithDrains(600000, 55), 90, 120)
	if g.FocusAmount <= 0 {
		t.Errorf("focus amount = %v, want > 0 (tip: %q)", g.FocusAmount, g.FocusTip)
	}
}

// The landing site copies the grade bands for share cards. Keep both lists the same.
func TestGradeBandsMatchLanding(t *testing.T) {
	path := filepath.Join("..", "..", "..", "azaman-landing", "src", "lib", "grade-share.ts")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Skip("landing repo not next to the engine")
	}
	for _, b := range gradeBands {
		key := b.grade
		if strings.ContainsAny(key, "+") {
			key = `"` + key + `"`
		}
		want := key + `: "` + b.headline + `"`
		if !strings.Contains(string(src), want) {
			t.Errorf("grade-share.ts is missing %s (update GRADE_HEADLINES)", want)
		}
	}
}
