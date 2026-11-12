// parse-check measures how well the engine reads a statement file, printing counts only (no
// statement text), so it is safe to run on real statements and paste the output anywhere.
//
//	go run ./cmd/parse-check statement.pdf [more files...]
//	go run ./cmd/parse-check -password 1234 -bad 5 statement.pdf
//	go run ./cmd/parse-check -against statement.xlsx statement.pdf
//
// -against compares each file, row by row, with another export of the same statement. Banks offer
// PDF and Excel; Excel keeps every memo and amount in its own cell, so it is the answer key.
//
// The key number is "balance chain": the share of rows whose balance equals the previous balance
// plus credit minus debit. A statement read correctly scores ~100%.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"azaman-engine/internal/models"
	"azaman-engine/internal/parser"
)

// shape masks a string so its layout is visible but its content is not: letters become a/A,
// digits become 9.
func shape(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case unicode.IsDigit(r):
			return '9'
		case unicode.IsUpper(r):
			return 'A'
		case unicode.IsLetter(r):
			return 'a'
		}
		return r
	}, s)
}

type report struct {
	rows, noDate, noDesc, noAmount, both, noBalance int
	chainOK, chainTotal                             int
	descChars                                       int
	debit, credit                                   float64
	outOfOrder                                      int
	bad                                             []int // row indexes that break the balance chain
}

func measure(txs []models.RawTransaction) report {
	var r report
	r.rows = len(txs)
	// Statements run oldest-first or newest-first; score the chain both ways and keep the better.
	score := func(sign float64) (ok, total int, bad []int) {
		for i := 1; i < len(txs); i++ {
			prev, cur := txs[i-1], txs[i]
			if prev.Balance == 0 || cur.Balance == 0 {
				continue
			}
			total++
			move := cur.Credit - cur.Debit
			if sign < 0 {
				move = -(prev.Credit - prev.Debit)
			}
			if math.Abs(prev.Balance+move-cur.Balance) < 0.011 {
				ok++
			} else {
				bad = append(bad, i)
			}
		}
		return
	}
	okF, totF, badF := score(1)
	okB, totB, badB := score(-1)
	r.chainOK, r.chainTotal, r.bad = okF, totF, badF
	if okB > okF {
		r.chainOK, r.chainTotal, r.bad = okB, totB, badB
	}
	for i, tx := range txs {
		if tx.ParsedDate.IsZero() {
			r.noDate++
		}
		if strings.TrimSpace(tx.Description) == "" {
			r.noDesc++
		}
		if tx.Debit == 0 && tx.Credit == 0 {
			r.noAmount++
		}
		if tx.Debit > 0 && tx.Credit > 0 {
			r.both++
		}
		if tx.Balance == 0 {
			r.noBalance++
		}
		if i > 0 && !tx.ParsedDate.IsZero() && !txs[i-1].ParsedDate.IsZero() {
			if tx.ParsedDate.Before(txs[i-1].ParsedDate) {
				r.outOfOrder++
			}
		}
		r.descChars += len(tx.Description)
		r.debit += tx.Debit
		r.credit += tx.Credit
	}
	// A newest-first statement has every pair "out of order"; report the smaller side.
	if r.outOfOrder > r.rows/2 {
		r.outOfOrder = r.rows - 1 - r.outOfOrder
	}
	return r
}

func pct(a, b int) string {
	if b == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%% (%d/%d)", 100*float64(a)/float64(b), a, b)
}

func main() {
	password := flag.String("password", "", "PDF password")
	showBad := flag.Int("bad", 0, "print the masked shape of this many rows that break the balance chain")
	against := flag.String("against", "", "another export of the same statement to compare with, row by row")
	flag.Parse()

	var key []models.RawTransaction
	if *against != "" {
		var err error
		if key, _, _, err = parse(*against, *password); err != nil {
			fmt.Println("cannot read -against file:", err)
			os.Exit(1)
		}
	}

	for n, path := range flag.Args() {
		txs, bank, check, err := parse(path, *password)
		fmt.Printf("file %d (%s) bank=%q\n", n+1, strings.ToLower(filepath.Ext(path)), bank)
		if err != nil {
			fmt.Println("  ERROR:", err)
			continue
		}
		r := measure(txs)
		avg := 0
		if r.rows > 0 {
			avg = r.descChars / r.rows
		}
		fmt.Printf("  rows            %d\n", r.rows)
		fmt.Printf("  balance chain   %s\n", pct(r.chainOK, r.chainTotal))
		fmt.Printf("  no date         %d\n", r.noDate)
		fmt.Printf("  no description  %d   (average length %d chars)\n", r.noDesc, avg)
		fmt.Printf("  no amount       %d\n", r.noAmount)
		fmt.Printf("  debit+credit    %d   (both set on one row)\n", r.both)
		fmt.Printf("  no balance      %d\n", r.noBalance)
		fmt.Printf("  date disorder   %d\n", r.outOfOrder)
		if check.Checked {
			fmt.Printf("  printed totals  ok=%v rows %d/%d debit %.2f/%.2f credit %.2f/%.2f (parsed/stated)\n",
				check.OK, check.ParsedRows, check.StatedRows, check.ParsedDebit, check.StatedDebit, check.ParsedCredit, check.StatedCredit)
		} else {
			fmt.Println("  printed totals  not found")
		}
		for i := 0; i < *showBad && i < len(r.bad); i++ {
			tx := txs[r.bad[i]]
			fmt.Printf("  bad row %d: date=%q desc=%q debit=%v credit=%v balance=%v\n", r.bad[i],
				shape(tx.Date), shape(tx.Description), tx.Debit != 0, tx.Credit != 0, tx.Balance != 0)
		}
		if key != nil {
			compare(txs, key)
		}
		fmt.Println()
	}
}

func parse(path, password string) (txs []models.RawTransaction, bank string, check models.StatementCheck, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", check, err
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		return parser.ParsePDF(data, password, parser.DetectBankFromFilename(filepath.Base(path)))
	case ".xlsx", ".xls":
		sheet, err := parser.ParseXLSX(data, "")
		return sheet.Txs, sheet.Bank, sheet.Check, err
	}
	sheet, err := parser.ParseCSV(bytes.NewReader(data), "")
	return sheet.Txs, sheet.Bank, sheet.Check, err
}

// compare counts, field by field, the rows that differ from the answer key.
func compare(txs, key []models.RawTransaction) {
	fmt.Printf("  against key     rows %d vs %d\n", len(txs), len(key))
	n := min(len(txs), len(key))
	norm := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	var date, debit, credit, balance, memo, ref, shown int
	for i := 0; i < n; i++ {
		a, b := txs[i], key[i]
		if !a.ParsedDate.Equal(b.ParsedDate) {
			date++
		}
		if math.Abs(a.Debit-b.Debit) > 0.001 {
			debit++
		}
		if math.Abs(a.Credit-b.Credit) > 0.001 {
			credit++
		}
		if math.Abs(a.Balance-b.Balance) > 0.001 {
			balance++
		}
		if norm(a.Reference) != norm(b.Reference) {
			ref++
		}
		if norm(a.Description) != norm(b.Description) {
			memo++
			if shown < 5 {
				shown++
				fmt.Printf("    row %d memo\n      got %s\n      key %s\n", i, shape(norm(a.Description)), shape(norm(b.Description)))
			}
		}
	}
	fmt.Printf("  rows differing  date=%d debit=%d credit=%d balance=%d memo=%d reference=%d (of %d)\n",
		date, debit, credit, balance, memo, ref, n)
}
