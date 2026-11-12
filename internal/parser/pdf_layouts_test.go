package parser

import (
	"math"
	"testing"

	"github.com/ledongthuc/pdf"
)

// A landscape statement draws upright-looking text with a quarter-turn matrix: page x and y swap.
// uprightPosition must put columns back along x and lines down y.
func TestUprightPositionUndoesQuarterTurn(t *testing.T) {
	// Text rotated 90 degrees counter-clockwise: reading direction is +y, "down the page" is +x.
	quarter := func(pageX, pageY float64) [6]float64 { return [6]float64{0, 1, -1, 0, pageX, pageY} }

	dateX, dateY := uprightPosition(quarter(40, 53))      // first row, date column
	remarkX, remarkY := uprightPosition(quarter(40, 600)) // first row, remarks column
	nextX, nextY := uprightPosition(quarter(94, 53))      // second row, date column

	if dateY != remarkY {
		t.Errorf("cells of one row must share a line: y=%v and y=%v", dateY, remarkY)
	}
	if !(remarkX > dateX) {
		t.Errorf("remarks column must be right of the date column: x=%v vs x=%v", remarkX, dateX)
	}
	if !(nextY < dateY) || nextX != dateX {
		t.Errorf("the second row must sit below the first, same column: (%v,%v) vs (%v,%v)", nextX, nextY, dateX, dateY)
	}

	// Upright and slanted text is left alone.
	if x, y := uprightPosition([6]float64{1, 0, 0, 1, 12, 34}); x != 12 || y != 34 {
		t.Errorf("upright text moved to (%v,%v)", x, y)
	}
	s := math.Sqrt2 / 2
	if x, y := uprightPosition([6]float64{s, s, -s, s, 12, 34}); x != 12 || y != 34 {
		t.Errorf("diagonal watermark moved to (%v,%v)", x, y)
	}
}

// Kuda-style table: "Money In" / "Money out" instead of debit and credit, a "To / From" column
// that names the other party, and a date with a time.
func TestParseTablePDFMoneyInOutColumns(t *testing.T) {
	y := 700.0
	page := []pdf.Text{
		run(30, 740, "Total Money In"), run(120, 740, "₦50,000.00"),
		run(30, 730, "Total Money Out"), run(120, 730, "₦12,500.00"),
		run(30, y, "Date/Time"), run(110, y, "Money In"), run(170, y, "Money out"), run(230, y, "Category"),
		run(290, y, "To / From"), run(380, y, "Description"), run(470, y, "Balance"),

		run(30, 690, "01/03/26 08:15:22"), run(110, 690, "50,000.00"), run(230, 690, "inward transfer"),
		run(290, 690, "Tunde Bakare"), run(380, 690, "salary march"), run(470, 690, "50,000.00"),

		run(30, 680, "02/03/26 19:40:03"), run(170, 680, "12,500.00"), run(230, 680, "outward transfer"),
		run(290, 680, "Golden Crust Bakery"), run(380, 680, "bread order"), run(470, 680, "37,500.00"),
	}
	txs, stated := parseTablePDF([][]pdf.Text{page}, BankKuda, "NGN")
	if len(txs) != 2 {
		t.Fatalf("expected 2 transactions, got %d: %+v", len(txs), txs)
	}
	if txs[0].Credit != 50000 || txs[0].Debit != 0 || txs[1].Debit != 12500 || txs[1].Credit != 0 {
		t.Errorf("money in/out not mapped to credit/debit: %+v", txs)
	}
	if txs[1].Description != "Golden Crust Bakery bread order" {
		t.Errorf("description should carry the other party then the narration, got %q", txs[1].Description)
	}
	if txs[1].ParsedDate.Hour() != 19 || txs[1].ParsedDate.Day() != 2 {
		t.Errorf("date with time misread: %v", txs[1].ParsedDate)
	}
	if stated == nil || stated.StatedCredit != 50000 || stated.StatedDebit != 12500 {
		t.Fatalf("printed totals not read: %+v", stated)
	}
	if check := CheckTotals(txs, stated); !check.OK {
		t.Errorf("parsed totals should match the printed ones: %+v", check)
	}
}

// One signed "Amount" column, newest transaction first: the direction of each row comes from
// its sign, a type column, or failing both the running balance read from the bottom up.
func TestParseTablePDFSingleAmountNewestFirst(t *testing.T) {
	y := 700.0
	page := []pdf.Text{
		run(30, y, "Transaction Date"), run(120, y, "Transaction Detail"), run(300, y, "Type"),
		run(360, y, "Amount"), run(430, y, "Balance"),

		// newest first: 10,000 in, then 2,000 out, then 500 out (unsigned, no type)
		run(30, 690, "05 Mar 2026 10:00"), run(120, 690, "Airtime purchase"), run(360, 690, "500.00"), run(430, 690, "7,500.00"),
		run(30, 680, "04 Mar 2026 09:00"), run(120, 680, "Transfer to Kemi"), run(300, 680, "Debit"), run(360, 680, "2,000.00"), run(430, 680, "8,000.00"),
		run(30, 670, "03 Mar 2026 08:00"), run(120, 670, "Received from Seun"), run(360, 670, "+10,000.00"), run(430, 670, "10,000.00"),
	}
	txs, _ := parseTablePDF([][]pdf.Text{page}, BankPalmPay, "NGN")
	if len(txs) != 3 {
		t.Fatalf("expected 3 transactions, got %d: %+v", len(txs), txs)
	}
	if txs[0].Debit != 500 || txs[0].Credit != 0 {
		t.Errorf("unsigned amount must take its direction from the balance (a 500 debit): %+v", txs[0])
	}
	if txs[1].Debit != 2000 {
		t.Errorf("type column says Debit: %+v", txs[1])
	}
	if txs[2].Credit != 10000 {
		t.Errorf("signed amount is a credit: %+v", txs[2])
	}
	if txs[0].Description != "Airtime purchase" {
		t.Errorf("\"Transaction Detail\" is the narration column, got %q", txs[0].Description)
	}
}

// Remarks that wrap past the bottom of a page continue at the top of the next, above its first
// date. They belong to the previous page's last transaction, and the footer does not.
func TestParseTablePDFRemarksContinueOnNextPage(t *testing.T) {
	page1 := append(header(700),
		run(30, 680, "08-Aug-2026"), run(200, 680, "3,000.00"), run(300, 680, "2,000.00"), run(420, 680, "RENT FOR THE"),
		run(30, 660, "09-Aug-2026"), run(200, 660, "50.00"), run(300, 660, "1,950.00"), run(420, 660, "NIP TRANSFER TO"),
		run(420, 654, "GOLDEN CRUST"),
		run(420, 600, "Guaranty Trust Bank footer text"), // far below: not part of the last row
	)
	page2 := []pdf.Text{
		run(420, 700, "BAKERY LTD"), // tail of page 1's last transaction
		run(30, 680, "10-Aug-2026"), run(250, 680, "9,000.00"), run(300, 680, "10,950.00"), run(420, 680, "SALARY"),
		run(30, 660, "11-Aug-2026"), run(200, 660, "950.00"), run(300, 660, "10,000.00"), run(420, 660, "POS PURCHASE"),
	}
	txs, _ := parseTablePDF([][]pdf.Text{page1, page2}, BankGTB, "NGN")
	if len(txs) != 4 {
		t.Fatalf("expected 4 transactions, got %d: %+v", len(txs), txs)
	}
	if got, want := txs[1].Description, "NIP TRANSFER TO GOLDEN CRUST BAKERY LTD"; got != want {
		t.Errorf("remarks split by a page break: got %q, want %q", got, want)
	}
	if txs[2].Description != "SALARY" {
		t.Errorf("the next page's first row must not take the carried-over line: %q", txs[2].Description)
	}
}

func TestLeadingDate(t *testing.T) {
	cases := []struct{ cell, date, rest string }{
		{"08-Aug-2026", "08-Aug-2026", ""},
		{"08-Aug-2026 08-Aug-2026", "08-Aug-2026", ""}, // trans date and value date drawn as one cell
		{"01/03/26 08:15:22", "01/03/26 08:15:22", ""},
		{"05 Mar 2026 10:00 Airtime purchase", "05 Mar 2026 10:00", "Airtime purchase"},
		{"2026-03-05T10:00:00", "2026-03-05T10:00:00", ""},
	}
	for _, c := range cases {
		date, rest, ok := leadingDate(c.cell)
		if !ok || date != c.date || rest != c.rest {
			t.Errorf("leadingDate(%q) = %q, %q, %v; want %q, %q", c.cell, date, rest, ok, c.date, c.rest)
		}
	}
	for _, notDate := range []string{"Opening Balance", "5,000.00", "", "Page 2 of 9"} {
		if _, _, ok := leadingDate(notDate); ok {
			t.Errorf("leadingDate(%q) should not find a date", notDate)
		}
	}
}

func TestDetectBankIgnoresCounterpartyNames(t *testing.T) {
	// GTBank letterhead is an image; narrations name other banks, the header names the columns.
	text := "Statement Period :01-Aug-2026 to 31-Aug-2026\nTrans. Date Value. Date Reference Debits Credits Balance Originating Branch Remarks\n" +
		"08-Aug-2026 NIP TRANSFER TO OPAY DIGITAL SERVICES LIMITED - CHINEDU\n09-Aug-2026 TRANSFER TO PALMPAY - KEMI\n10-Aug-2026 TRANSFER TO OPAY - SEUN\n"
	if got := DetectBank(text); got != BankGTB {
		t.Errorf("DetectBank = %q, want %q", got, BankGTB)
	}
}

func TestParseDateFormats(t *testing.T) {
	good := map[string]string{
		"02-Jan-2026":               "2026-01-02 00:00:00",
		"02-JAN-26":                 "2026-01-02 00:00:00",
		"2/1/2026":                  "2026-01-02 00:00:00",
		"02/01/26 08:15:22":         "2026-01-02 08:15:22",
		"02 Jan 2026 10:15":         "2026-01-02 10:15:00",
		"02 Jan 2026, 3:04 PM":      "2026-01-02 15:04:00",
		"2nd January 2026":          "2026-01-02 00:00:00",
		"Jan 2, 2026 3:04 pm":       "2026-01-02 15:04:00",
		"Friday, January 2, 2026":   "2026-01-02 00:00:00",
		"2026-01-02T15:04:05Z":      "2026-01-02 15:04:05",
		"2026-01-02 15:04:05.000":   "2026-01-02 15:04:05",
		"2026-01-02T15:04:05+01:00": "2026-01-02 15:04:05",
		"02.01.2026":                "2026-01-02 00:00:00",
		"12 Sept 2026":              "2026-09-12 00:00:00",
		"01/15/2026":                "2026-01-15 00:00:00", // month first: only possible reading
	}
	for in, want := range good {
		got, ok := ParseDate(in)
		if !ok || got.Format("2006-01-02 15:04:05") != want {
			t.Errorf("ParseDate(%q) = %v, %v; want %s", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "Opening Balance", "12345678", "99999999", "5,000.00", "20260102345", "02-Jan-1850"} {
		if got, ok := ParseDate(bad); ok {
			t.Errorf("ParseDate(%q) should fail, got %v", bad, got)
		}
	}
}
