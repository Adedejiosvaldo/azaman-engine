package analyzer

import (
	"testing"
	"time"

	"azaman-engine/internal/models"
)

func TestAnalyzeMidnight(t *testing.T) {
	txs := []models.DecodedTransaction{
		{
			Merchant:       "Chowdeck Delivery",
			Amount:         -8500,
			Debit:          8500,
			Category:       "Late Night Food & Lounges",
			Classification: "Personal Discretionary",
			Flags:          models.TransactionFlags{IsMidnightLeak: true},
		},
		{
			Merchant:       "Bolt Ride Hailing",
			Amount:         -6200,
			Debit:          6200,
			Category:       "Commute & Transit",
			Classification: "Personal Discretionary",
			Flags:          models.TransactionFlags{IsMidnightLeak: true},
		},
		{
			Merchant:       "AWS Cloud",
			Amount:         -50000,
			Debit:          50000,
			Category:       "Tech & Cloud Infrastructure",
			Classification: "Tax-Deductible Business Expense",
			Flags:          models.TransactionFlags{IsMidnightLeak: false},
		},
	}

	report := AnalyzeMidnight(txs, 20000)
	if report.Count != 2 {
		t.Errorf("expected 2 midnight leaks, got %d", report.Count)
	}
	expectedAmount := 8500.0 + 6200.0
	if report.TotalAmount != expectedAmount {
		t.Errorf("expected total amount %f, got %f", expectedAmount, report.TotalAmount)
	}
	if len(report.TopMerchants) != 2 {
		t.Errorf("expected 2 top merchants, got %d", len(report.TopMerchants))
	}
}

func TestAnalyzeUrgent2k(t *testing.T) {
	txs := []models.DecodedTransaction{
		{
			Merchant:       "Urgent 2k Favors",
			RawDescription: "Transfer to Mum for upkeep",
			Amount:         -5000,
			Debit:          5000,
			Category:       "Family & Favors",
			Flags:          models.TransactionFlags{IsUrgent2k: true},
		},
		{
			Merchant:       "Transfer to Friend",
			RawDescription: "Urgent 2k for transport",
			Amount:         -2000,
			Debit:          2000,
			Category:       "Family & Favors",
			Flags:          models.TransactionFlags{IsUrgent2k: true},
		},
	}

	report := AnalyzeUrgent2k(txs, 14.0) // 2 weeks
	if report.Count != 2 {
		t.Errorf("expected 2 urgent 2k transactions, got %d", report.Count)
	}
	if report.TotalAmount != 7000.0 {
		t.Errorf("expected 7000 total, got %f", report.TotalAmount)
	}
	if report.AverageTransferSize != 3500.0 {
		t.Errorf("expected 3500 avg size, got %f", report.AverageTransferSize)
	}
	if report.FamilyVsErrandSplit.FamilyAmount != 5000.0 {
		t.Errorf("expected 5000 family amount, got %f", report.FamilyVsErrandSplit.FamilyAmount)
	}
	if report.FamilyVsErrandSplit.ErrandFavorsAmount != 2000.0 {
		t.Errorf("expected 2000 errand amount, got %f", report.FamilyVsErrandSplit.ErrandFavorsAmount)
	}
}

func TestAnalyzeDuplicates(t *testing.T) {
	now := time.Now()
	txs := []models.DecodedTransaction{
		{
			Merchant:   "Choplife Bar & Grill",
			Amount:     -15000,
			Debit:      15000,
			Date:       now.Format(time.RFC3339),
			ParsedDate: now,
		},
		{
			Merchant:   "Choplife Bar & Grill",
			Amount:     -15000,
			Debit:      15000,
			Date:       now.Add(10 * time.Minute).Format(time.RFC3339),
			ParsedDate: now.Add(10 * time.Minute),
		},
		{
			Merchant:   "Bolt Ride Hailing",
			Amount:     -3000,
			Debit:      3000,
			Date:       now.Format(time.RFC3339),
			ParsedDate: now,
		},
	}

	report, enriched := AnalyzeDuplicates(txs)
	if len(report.Pairs) != 1 {
		t.Fatalf("expected 1 duplicate pair, got %d", len(report.Pairs))
	}
	if report.TotalPotentialRefund != 15000.0 {
		t.Errorf("expected 15000 refund, got %f", report.TotalPotentialRefund)
	}
	if !enriched[0].Flags.IsDuplicateDebit || !enriched[1].Flags.IsDuplicateDebit {
		t.Errorf("expected both duplicate debits to be flagged")
	}
}

func TestCalculateRunway(t *testing.T) {
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	txs := []models.DecodedTransaction{
		{
			Amount:     1000000,
			Credit:     1000000,
			ParsedDate: startDate,
			Balance:    1000000,
		},
		{
			Amount:     -300000,
			Debit:      300000,
			ParsedDate: endDate,
			Balance:    700000,
		},
	}

	summary := CalculateRunway(txs, 30.0)
	if summary.TotalInflow != 1000000.0 {
		t.Errorf("expected 1M inflow, got %f", summary.TotalInflow)
	}
	if summary.TotalOutflow != 300000.0 {
		t.Errorf("expected 300k outflow, got %f", summary.TotalOutflow)
	}
	if summary.NetRetained != 700000.0 {
		t.Errorf("expected 700k net retained, got %f", summary.NetRetained)
	}
	if summary.ProfitMarginPct != 70.0 {
		t.Errorf("expected 70%% margin, got %f", summary.ProfitMarginPct)
	}
	// Monthly burn = 300,000, Current balance = 700,000 -> Runway = 700k / 300k = ~2.3 months
	if summary.RunwayMonths < 2.0 || summary.RunwayMonths > 2.5 {
		t.Errorf("expected ~2.3 runway months, got %f", summary.RunwayMonths)
	}
}
