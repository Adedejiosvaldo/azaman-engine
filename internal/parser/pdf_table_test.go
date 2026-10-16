package parser

import (
	"testing"

	"github.com/ledongthuc/pdf"
)

// run builds a text run the way pageRuns does (width estimated from length).
func run(x, y float64, s string) pdf.Text {
	return pdf.Text{X: x, Y: y, FontSize: 6, S: s, W: float64(len([]rune(s))) * 3}
}

func header(y float64) []pdf.Text {
	return []pdf.Text{
		run(30, y, "Trans. Date"), run(80, y, "Value Date"), run(130, y, "Reference"),
		run(200, y, "Debits"), run(250, y, "Credits"), run(300, y, "Balance"),
		run(350, y, "Originating Branch"), run(420, y, "Remarks"),
	}
}

// Real GTBank layout: rows are top-aligned (the date shares a line with the first remarks line),
// long remarks wrap downward, and an Originating Branch column repeats the branch on every row.
func gtbPage() []pdf.Text {
	page := []pdf.Text{run(30, 720, "29-JUL-26 TO 28-AUG-26")} // statement period above the table
	page = append(page, header(700)...)
	page = append(page,
		run(300, 690, "5,000.00"), run(420, 690, "Opening Balance"),

		// NIP transfer whose remarks wrap onto two more lines
		run(30, 680, "08-Aug-2026"), run(200, 680, "3,000.00"), run(300, 680, "2,000.00"),
		run(350, 680, "AKIN ADESOLA"), run(420, 680, "NIP TRANSFER TO"),
		run(420, 674, "OPAY - CHINEDU OLUMIDE"),
		run(420, 668, "BALOGUN"),

		// Short fee row directly after it
		run(30, 660, "08-Aug-2026"), run(200, 660, "50.00"), run(300, 660, "1,950.00"),
		run(350, 660, "AKIN ADESOLA"), run(420, 660, "Commission on NIP Transfer"),

		// Amount drawn under the Debits header by mistake; the balance proves it's a credit
		run(30, 650, "10-Aug-2026"), run(200, 650, "30,000.00"), run(300, 650, "31,950.00"),
		run(350, 650, "AKIN ADESOLA"), run(420, 650, "TRANSFER FROM CHINEDU"),
		run(420, 644, "OPAY"),
	)
	return page
}

func TestParseTablePDFGTBankTopAligned(t *testing.T) {
	txs := parseTablePDF([][]pdf.Text{gtbPage()}, BankGTB, "NGN")
	if len(txs) != 3 {
		t.Fatalf("expected 3 transactions (no header/period/opening rows), got %d: %+v", len(txs), txs)
	}

	want := []struct {
		desc          string
		debit, credit float64
	}{
		{"NIP TRANSFER TO OPAY - CHINEDU OLUMIDE BALOGUN", 3000, 0},
		{"Commission on NIP Transfer", 50, 0},
		{"TRANSFER FROM CHINEDU OPAY", 0, 30000}, // reconciled from the balance change
	}
	for i, w := range want {
		if txs[i].Description != w.desc {
			t.Errorf("tx%d remarks: got %q, want %q (wrapped lines must stay with their row, branch excluded)", i+1, txs[i].Description, w.desc)
		}
		if txs[i].Debit != w.debit || txs[i].Credit != w.credit {
			t.Errorf("tx%d amounts: debit=%v credit=%v, want %v/%v", i+1, txs[i].Debit, txs[i].Credit, w.debit, w.credit)
		}
	}
}

// Some banks centre the date vertically in a tall cell, so remarks sit above and below it.
func TestParseTablePDFCentredLayout(t *testing.T) {
	page := header(700)
	page = append(page,
		run(420, 686, "POSWEB PURCHASE"),
		run(30, 680, "10-Aug-2026"), run(200, 680, "3,558.65"), run(300, 680, "1,441.35"),
		run(420, 674, "ANTHROPIC CLAUDE"),

		run(420, 662, "TRANSFER FROM"),
		run(30, 656, "11-Aug-2026"), run(250, 656, "1,000.00"), run(300, 656, "2,441.35"),
		run(420, 650, "OPAY"),
	)
	txs := parseTablePDF([][]pdf.Text{page}, "", "NGN")
	if len(txs) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(txs))
	}
	if txs[0].Description != "POSWEB PURCHASE ANTHROPIC CLAUDE" || txs[1].Description != "TRANSFER FROM OPAY" {
		t.Errorf("centred remarks misassigned: %q / %q", txs[0].Description, txs[1].Description)
	}
}

func TestParseTablePDFNeedsHeader(t *testing.T) {
	page := []pdf.Text{run(30, 676, "10-Aug-2026"), run(200, 676, "3,558.65")}
	if txs := parseTablePDF([][]pdf.Text{page}, "", "NGN"); len(txs) != 0 {
		t.Errorf("without a Debit/Credit/Balance header the table parser must not guess, got %d rows", len(txs))
	}
}

// What GTBank actually does: the date sits on the SECOND remarks line, lines inside a cell are
// 6pt apart and rows are separated by extra padding (10pt). Date-row windows get this wrong.
func TestParseTablePDFDateOnSecondRemarksLine(t *testing.T) {
	page := header(700)
	page = append(page,
		run(420, 680, "POSWEB PURCHASE TRANSACTION WEB PUR"),
		run(30, 674, "03-Aug-2026"), run(200, 674, "900.00"), run(300, 674, "4,100.00"), run(420, 674, "3PG00012 OPAY CHARGE"),
		run(420, 668, "539983***9983"),

		run(420, 658, "POSWEB PURCHASE TRANSACTION WEB PUR"),
		run(30, 652, "10-Aug-2026"), run(200, 652, "3,558.65"), run(300, 652, "541.35"), run(420, 652, "ANTHROPIC CLAUDE SUB"),
		run(420, 646, "G0A3LMSE"),

		run(30, 636, "10-Aug-2026"), run(200, 636, "50.00"), run(300, 636, "491.35"), run(420, 636, "Commission on NIP Transfer"),
	)
	txs := parseTablePDF([][]pdf.Text{page}, BankGTB, "NGN")
	want := []string{
		"POSWEB PURCHASE TRANSACTION WEB PUR 3PG00012 OPAY CHARGE 539983***9983",
		"POSWEB PURCHASE TRANSACTION WEB PUR ANTHROPIC CLAUDE SUB G0A3LMSE",
		"Commission on NIP Transfer",
	}
	if len(txs) != len(want) {
		t.Fatalf("expected %d transactions, got %d", len(want), len(txs))
	}
	for i, w := range want {
		if txs[i].Description != w {
			t.Errorf("tx%d: got %q, want %q", i+1, txs[i].Description, w)
		}
	}
}
