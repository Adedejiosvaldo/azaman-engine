package parser

import (
	"testing"
	"time"

	"azaman-engine/internal/models"
	"github.com/ledongthuc/pdf"
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

// opayBody builds description-column rows the way the PDF library returns them.
func opayBody(lines ...struct {
	y int64
	s string
}) pdf.Rows {
	var rows pdf.Rows
	for _, l := range lines {
		rows = append(rows, &pdf.Row{Position: l.y, Content: pdf.TextHorizontal{{X: 177, S: l.s}}})
	}
	return rows
}

type opayLine = struct {
	y int64
	s string
}

var opayTestCols = []opayColumn{{colTransDate, 71}, {colRemarks, 177}, {colDebit, 306}, {colReference, 479}}

// A five-line memo reaches past the halfway point to the next date. Its lines are 12 apart; the
// next transaction's memo starts 21 below. Spacing, not distance to the date, decides ownership.
func TestOPayDescriptionsLongMemoKeepsAllItsLines(t *testing.T) {
	body := opayBody(
		opayLine{505, "Transfer from TUNDE BAKARE | "}, opayLine{493, "Kuda MFB | 858****054 | "},
		opayLine{481, "payment for the generator "}, opayLine{469, "repair and two weeks of "}, opayLine{457, "diesel"},
		opayLine{436, "Airtime | 08030000000"},
	)
	per, lead, next := opayDescriptions(body, opayTestCols, []float64{481, 436})
	if got := joinWrapped(per[0]); got != "Transfer from TUNDE BAKARE | Kuda MFB | 858****054 | payment for the generator repair and two weeks of diesel" {
		t.Errorf("long memo lost lines: %q", got)
	}
	if got := joinWrapped(per[1]); got != "Airtime | 08030000000" {
		t.Errorf("next memo took a line that is not its own: %q", got)
	}
	if lead != nil || next != nil {
		t.Errorf("nothing crosses a page here: lead=%v next=%v", lead, next)
	}
}

// OPay splits a row across pages: the memo's first line ends one page, its date starts the next.
func TestOPayDescriptionsRowSplitByPageBreak(t *testing.T) {
	pageEnd := opayBody(
		opayLine{77, "Transfer to KEMI BAKARE | "}, opayLine{65, "OPay | 8031000027"},
		opayLine{44, "Transfer to GOLDEN CRUST BAKERY | "}, // its date is on the next page
	)
	per, lead, next := opayDescriptions(pageEnd, opayTestCols, []float64{71})
	if got := joinWrapped(per[0]); got != "Transfer to KEMI BAKARE | OPay | 8031000027" {
		t.Errorf("last transaction on the page took the next one's line: %q", got)
	}
	if lead != nil || joinWrapped(next) != "Transfer to GOLDEN CRUST BAKERY |" {
		t.Errorf("the stray line is owed to the next page: lead=%v next=%v", lead, next)
	}

	// The other way round: the date is on the first page, the memo's tail tops the next one.
	pageStart := opayBody(opayLine{748, "for the wedding cake"}, opayLine{700, "Mobile Data"})
	per, lead, _ = opayDescriptions(pageStart, opayTestCols, []float64{700})
	if joinWrapped(lead) != "for the wedding cake" || joinWrapped(per[0]) != "Mobile Data" {
		t.Errorf("a memo tail above the first date finishes the previous page: lead=%v first=%v", lead, per[0])
	}
}

func TestJoinWrappedMidWordBreak(t *testing.T) {
	// A line that broke between words keeps its trailing space; one that broke mid-word has none.
	if got := joinWrapped([]string{"Transfer from CHIDI TUN", "DE | Kuda MFB | ", "salary"}); got != "Transfer from CHIDI TUNDE | Kuda MFB | salary" {
		t.Errorf("got %q", got)
	}
}
