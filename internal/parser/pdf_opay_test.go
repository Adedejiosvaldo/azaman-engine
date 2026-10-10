package parser

import (
	"testing"
	"time"

	"azaman-engine/internal/models"
)

func TestOPayColumnAtUsesHeaderPositions(t *testing.T) {
	// Wallet and savings sections place the same columns at different x positions
	wallet := []opayColumn{{colTransDate, 71}, {colRemarks, 177}, {colDebit, 306}, {colCredit, 344}, {colBalance, 382}, {colBranch, 423}}
	savings := []opayColumn{{colTransDate, 71}, {colRemarks, 195}, {colDebit, 257}, {colCredit, 302}, {colBalance, 346}, {colBranch, 395}}
	cases := []struct {
		cols []opayColumn
		x    float64
		want string
	}{
		{wallet, 306, colDebit}, {wallet, 344, colCredit}, {wallet, 382, colBalance},
		{savings, 302, colCredit}, {savings, 346, colBalance}, {savings, 257, colDebit},
	}
	for _, c := range cases {
		if got := opayColumnAt(c.cols, c.x); got != c.want {
			t.Errorf("x=%.0f: got %q, want %q", c.x, got, c.want)
		}
	}
}

func TestFillCardFundedTransfers(t *testing.T) {
	at := func(s string) time.Time { v, _ := time.Parse("15:04:05", s); return v }
	txs := []models.RawTransaction{
		{Description: "Transfer to BLESSING ADEWUNMI | OPay", ParsedDate: at("10:54:27")},
		{Description: "Add Money-Bank Card | Guaranty Trust Bank", Credit: 1500, ParsedDate: at("10:54:32")},
		{Description: "Mobile Data | Glo", ParsedDate: at("14:52:36")},
		{Description: "Add Money-Bank Card | Guaranty Trust Bank", Credit: 178, ParsedDate: at("14:56:11")},
	}
	fillCardFundedTransfers(txs)
	if txs[0].Debit != 1500 {
		t.Errorf("card-funded transfer debit = %.2f, want 1500", txs[0].Debit)
	}
	if txs[2].Debit != 0 {
		t.Errorf("top-up minutes later must not be paired, got debit %.2f", txs[2].Debit)
	}
}

func TestCheckTotals(t *testing.T) {
	txs := []models.RawTransaction{{Debit: 100}, {Credit: 250}}
	if c := CheckTotals(txs, nil); c.Checked {
		t.Error("no stated totals must not be reported as checked")
	}
	ok := CheckTotals(txs, &models.StatementCheck{StatedDebit: 100, StatedCredit: 250, StatedRows: 2})
	if !ok.Checked || !ok.OK {
		t.Errorf("matching totals: %+v", ok)
	}
	bad := CheckTotals(txs, &models.StatementCheck{StatedDebit: 100, StatedCredit: 250, StatedRows: 3})
	if bad.OK {
		t.Error("missing rows must fail the check")
	}
}
