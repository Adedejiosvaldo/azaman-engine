package analyzer

import (
	"math"
	"strings"
	"testing"
	"time"

	"azaman-engine/internal/models"
	"azaman-engine/internal/shazam"
)

// A one-month OPay wallet paid for mostly from the holder's other banks, shaped like a real
// statement: every payment is swept out of OWealth, 10% goes to Spend & Save, and the wallet ends
// the day on ₦0.00. Names and numbers are made up.
func opayFundedStatement() []models.DecodedTransaction {
	type row struct {
		day, hour     int
		desc          string
		debit, credit float64
		balance       float64
	}
	rows := []row{
		{1, 9, "Transfer from TUNDE OLUMIDE BAKARE | Wema Bank PLC | 040****560 | ALAT NIP TRANSFER", 0, 100000, 100000},
		{1, 9, "Auto-save to OWealth Balance", 100000, 0, 0},
		{2, 10, "OWealth Withdrawal(Transaction Payment)", 0, 40000, 40000},
		{2, 10, "Transfer to KOLA MEDIA LIMITED | Access Bank | 1111111111 | Media Equipment", 40000, 0, 0},
		{3, 8, "Transfer from MR TUNDE BAKARE | Zenith Bank | 102****976 | Tf", 0, 50000, 50000},
		{3, 8, "Auto-save to OWealth Balance", 50000, 0, 0},
		{4, 11, "OWealth Withdrawal(Transaction Payment)", 0, 5500, 5500},
		{4, 11, "Third-Party Merchant Order | Chowdeck", 5000, 0, 500},
		{4, 11, "Spend & Save Deposit", 500, 0, 0},
		{5, 23, "OWealth Withdrawal(Transaction Payment)", 0, 3000, 3000},
		{5, 23, "Third-Party Merchant Order | Chowdeck", 3000, 0, 0},
		{6, 12, "Transfer from AWE MFB | Awe MFB | 194****318 | Transfer Payment", 0, 10000, 10000},
		{6, 12, "Transfer to TUNDE OLUMIDE BAKARE | MONIE POINT | 8031111111", 10000, 0, 0},
		{7, 9, "OWealth Withdrawal(Transaction Payment)", 0, 30000, 30000},
		{7, 9, "Transfer to PIGGYVEST/TUNDE BAKARE | POCKETAPP | 9328111111 | Rent", 30000, 0, 0},
		{29, 18, "OWealth Withdrawal(Transaction Payment)", 0, 900, 900},
		{29, 18, "Mobile Data | 8031****27 | MTN | 2.5GB 2 Day Plan", 900, 0, 0},
	}
	var txs []models.DecodedTransaction
	for _, r := range rows {
		when := time.Date(2026, 9, r.day, r.hour, 0, 0, 0, time.UTC)
		tx := shazam.MatchAndDecode(models.RawTransaction{Description: r.desc, Debit: r.debit, Credit: r.credit,
			Balance: r.balance, ParsedDate: when, Date: when.Format("02 Jan 2006 15:04:05"), Currency: "NGN"})
		tx.Account = "OPay"
		txs = append(txs, tx)
	}
	return txs
}

func TestOwnFundedStatementIsNotFailedOnMoneyKept(t *testing.T) {
	report := GenerateAudit(opayFundedStatement(), "OPay", "s1", "pdf", 0, time.Now(), "TUNDE OLUMIDE BAKARE")
	s := report.Summary

	if s.TotalInflow != 10000 {
		t.Errorf("income should be the Awe MFB payment only, got %v", s.TotalInflow)
	}
	// ₦150,000 in from Wema and Zenith, ₦10,000 back out to the holder's Moniepoint
	if s.OwnFunding != 140000 {
		t.Errorf("own funding = %v, want 140000", s.OwnFunding)
	}
	sources := strings.Join(s.OwnFundingSources, ",")
	if !strings.Contains(sources, "Wema Bank") || !strings.Contains(sources, "Zenith Bank") {
		t.Errorf("own funding sources = %v, want Wema Bank and Zenith Bank", s.OwnFundingSources)
	}

	for _, f := range report.Grade.Factors {
		switch f.Key {
		case "money_kept":
			if f.Applicable {
				t.Errorf("money kept should be left out of a statement funded from the holder's own accounts: %+v", f)
			}
			if !strings.Contains(f.Detail, "Wema Bank") || strings.Contains(f.Detail, "more than came in") {
				t.Errorf("money kept detail should point to the other accounts, got %q", f.Detail)
			}
		case "saving_habit":
			// Net saved ₦101,100 (₦70,600 left in OWealth, ₦30,000 PiggyVest, ₦500 Spend & Save)
			// over ₦150,000 in (income + own funding), not over the ₦10,000 income alone.
			if f.Value != 67.4 {
				t.Errorf("saving rate should use all money in, got %v%%", f.Value)
			}
		}
	}
	if report.Grade.Focus == "money_kept" {
		t.Errorf("focus tip should not ask to keep more when the money came from the holder's other accounts")
	}
}

func TestOPaySweepsAreNotCountedAsSavings(t *testing.T) {
	report := GenerateAudit(opayFundedStatement(), "OPay", "s1", "pdf", 0, time.Now(), "TUNDE OLUMIDE BAKARE")
	var saved float64
	for _, b := range report.SpendingMix.Buckets {
		for _, c := range b.TopCategories {
			if c.Category == "Savings deposits" {
				saved = c.Amount
			}
		}
	}
	// Gross deposits are ₦180,500 (two auto-saves, Spend & Save, PiggyVest); ₦79,400 came back out.
	if want := report.Internal.NetSaved; saved != want || saved > 110000 {
		t.Errorf("builders should hold net savings %v, got %v (gross deposits %v)", want, saved, report.Internal.SavingsDeposits)
	}
}

func TestZeroClosingBalanceCounts(t *testing.T) {
	txs := opayFundedStatement()
	report := GenerateAudit(txs, "OPay", "s1", "pdf", 0, time.Now(), "TUNDE OLUMIDE BAKARE")
	if report.Summary.CurrentBalance != 0 {
		t.Errorf("the wallet ends on ₦0.00, got balance %v", report.Summary.CurrentBalance)
	}
	accounts := []models.AccountSummary{{Label: "OPay"}}
	FillAccountTotals(accounts, report.Transactions)
	if accounts[0].ClosingBalance != 0 {
		t.Errorf("account closing balance should be ₦0.00, got %v", accounts[0].ClosingBalance)
	}

	// A file with no balance column still falls back to net retained.
	for i := range txs {
		txs[i].Balance = 0
	}
	report = GenerateAudit(txs, "OPay", "s1", "csv", 0, time.Now(), "TUNDE OLUMIDE BAKARE")
	if report.Summary.CurrentBalance != report.Summary.NetRetained {
		t.Errorf("no balance column: balance %v should equal net retained %v",
			report.Summary.CurrentBalance, report.Summary.NetRetained)
	}
}

func TestDaytimeDeliveryIsFoodNotLateNight(t *testing.T) {
	report := GenerateAudit(opayFundedStatement(), "OPay", "s1", "pdf", 0, time.Now(), "TUNDE OLUMIDE BAKARE")
	for _, tx := range report.Transactions {
		if !strings.Contains(tx.RawDescription, "Chowdeck") {
			continue
		}
		late := tx.ParsedDate.Hour() >= 22
		if late && (tx.Category != "Late Night Food & Lounges" || !tx.Flags.IsMidnightLeak) {
			t.Errorf("11 PM Chowdeck should stay a late-night leak, got %s", tx.Category)
		}
		if !late && (tx.Category != "Food & Dining" || tx.Flags.IsMidnightLeak) {
			t.Errorf("11 AM Chowdeck is lunch, got %s (midnight %v)", tx.Category, tx.Flags.IsMidnightLeak)
		}
	}
}

func TestEquipmentPurposeIsAPurchase(t *testing.T) {
	tx := shazam.MatchAndDecode(models.RawTransaction{
		Description: "Transfer to KOLA MEDIA LIMITED | Access Bank | 1111111111 | Media Equipment", Debit: 40000})
	if tx.Category != "Shopping & Retail" {
		t.Errorf("a transfer for equipment is a purchase, got %s", tx.Category)
	}
}

func TestShortStatementTaxNoteWarnsAboutScaling(t *testing.T) {
	report := GenerateAudit(opayFundedStatement(), "OPay", "s1", "pdf", 0, time.Now(), "TUNDE OLUMIDE BAKARE")
	if !strings.Contains(report.Tax.Note, "scaled to a year") {
		t.Errorf("a one-month tax estimate should say it was scaled up, got %q", report.Tax.Note)
	}
}

func TestRelativesAreFamilySupportNotLeaks(t *testing.T) {
	memo := shazam.MatchAndDecode(models.RawTransaction{
		Description: "Transfer to ADA OKORO | OPay | 8111111111 | for mum", Debit: 5000})
	if memo.Category != "Black Tax (Family Support)" || memo.Flags.IsUrgent2k {
		t.Errorf("a memo naming mum is family support, got %s (urgent %v)", memo.Category, memo.Flags.IsUrgent2k)
	}
	friend := shazam.MatchAndDecode(models.RawTransaction{
		Description: "Transfer to ADA OKORO | OPay | 8111111111 | for the week", Debit: 5000})
	if !friend.Flags.IsUrgent2k {
		t.Errorf("a support memo with no relative named stays an Urgent 2k favour, got %s", friend.Category)
	}
}

func TestOWealthPocketCountsTowardsRunway(t *testing.T) {
	report := GenerateAuditWithPockets(opayFundedStatement(), "OPay", "s1", "pdf", 0, time.Now(), "TUNDE OLUMIDE BAKARE", 70600)
	if report.Summary.CurrentBalance != 70600 {
		t.Errorf("balance should include the OWealth pocket, got %v", report.Summary.CurrentBalance)
	}
	want := math.Round(70600/report.Summary.WeightedMonthlyBurn*10) / 10
	if report.Summary.RunwayMonths != want || want == 0 {
		t.Errorf("runway = %v, want %v", report.Summary.RunwayMonths, want)
	}
}

func TestShortDataPlansAreNamed(t *testing.T) {
	txs := opayFundedStatement()
	for d := 8; d <= 26; d += 2 {
		when := time.Date(2026, 9, d, 8, 0, 0, 0, time.UTC)
		tx := shazam.MatchAndDecode(models.RawTransaction{Description: "Mobile Data | 8031****27 | MTN | 2.5GB 2 Day Plan",
			Debit: 900, ParsedDate: when, Date: when.Format("02 Jan 2006 15:04:05"), Currency: "NGN"})
		tx.Account = "OPay"
		txs = append(txs, tx)
	}
	report := GenerateAudit(txs, "OPay", "s1", "pdf", 0, time.Now(), "TUNDE OLUMIDE BAKARE")
	found := false
	for _, w := range report.Insights.Warnings {
		if strings.Contains(w, "₦9,900 went on 11 short data plans") {
			found = true
		}
	}
	if !found {
		t.Errorf("11 two-day plans should be named, warnings: %v", report.Insights.Warnings)
	}
}

func TestMonthlyNetCountsOwnFunding(t *testing.T) {
	report := GenerateAudit(opayFundedStatement(), "OPay", "s1", "pdf", 0, time.Now(), "TUNDE OLUMIDE BAKARE")
	m := report.Monthly.Months[0]
	if m.OwnFunding != 140000 {
		t.Errorf("month own funding = %v, want 140000", m.OwnFunding)
	}
	if want := math.Round((m.Inflow+m.OwnFunding-m.Outflow)*100) / 100; m.Net != want || m.Net <= 0 {
		t.Errorf("month net = %v, want %v (income + own funding - spend)", m.Net, want)
	}
}
