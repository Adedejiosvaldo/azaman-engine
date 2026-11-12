package parser

import (
	"bytes"
	"encoding/csv"
	"io"
	"math"
	"strings"
	"time"

	"azaman-engine/internal/models"
	"github.com/xuri/excelize/v2"
)

// Sheet is what a CSV or Excel statement yields: its transactions, plus what its heading rows say
// about the statement itself.
type Sheet struct {
	Txs    []models.RawTransaction
	Bank   string                // the bank passed in, or the one named in the heading rows
	Holder string                // account holder named in the heading rows, if any
	Check  models.StatementCheck // parsed totals against the printed ones, when printed
}

type columnIndices struct {
	DateIdx        int
	DescIdx        int
	PartyIdx       int
	DebitIdx       int
	CreditIdx      int
	AmountIdx      int
	BalanceIdx     int
	RefIdx         int
	TypeIdx        int
	HeaderRowIdx   int
	HeaderRowFound bool
}

// ParseCSV parses a bank statement CSV stream in-memory without disk I/O.
func ParseCSV(r io.Reader, bankName string) (Sheet, error) {
	// Read full content to determine separator (comma, semicolon, or tab)
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, r); err != nil {
		return Sheet{}, err
	}
	content := bytes.TrimPrefix(buf.Bytes(), []byte("\xef\xbb\xbf")) // Excel's UTF-8 marker
	if len(content) == 0 {
		return Sheet{Bank: bankName}, nil
	}

	// Detect delimiter
	sample := string(content[:int(math.Min(float64(len(content)), 2048))])
	delimiter := ','
	for _, d := range []rune{';', '\t', '|'} {
		if strings.Count(sample, string(d)) > strings.Count(sample, string(delimiter)) {
			delimiter = d
		}
	}

	reader := csv.NewReader(bytes.NewReader(content))
	reader.Comma = delimiter
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1 // Allow variable number of fields

	records, err := reader.ReadAll()
	if err != nil {
		return Sheet{}, err
	}
	return processRecords(records, bankName), nil
}

// ParseXLSX reads transactions from an Excel workbook entirely in-memory. The statement is the
// first sheet that holds transactions: a cover sheet before it is skipped, and later sheets are
// other accounts (OPay adds the OWealth savings account, whose movements the wallet already shows).
func ParseXLSX(data []byte, bankName string) (Sheet, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return Sheet{}, err
	}
	defer f.Close()

	best := Sheet{Bank: bankName}
	for _, name := range f.GetSheetList() {
		rows, err := f.GetRows(name)
		if err != nil {
			continue
		}
		if sheet := processRecords(rows, bankName); len(sheet.Txs) > 0 {
			return sheet, nil
		}
	}
	return best, nil
}

func processRecords(records [][]string, bankName string) Sheet {
	indices := detectHeaderIndices(records)

	startRow := 0
	heading := ""
	if indices.HeaderRowFound {
		startRow = indices.HeaderRowIdx + 1
		heading = headingText(records[:indices.HeaderRowIdx+1])
	} else {
		heading = headingText(records[:min(len(records), 12)])
	}

	sheet := Sheet{Bank: bankName, Holder: ExtractAccountHolder(heading)}
	if sheet.Bank == "" || sheet.Bank == BankGeneric {
		sheet.Bank = DetectBank(heading)
	}
	currency := DetectCurrency(heading)
	if sheet.Bank == BankGeegpay {
		currency = "USD"
	}

	cell := func(row []string, idx int) string {
		if idx >= 0 && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	for r := startRow; r < len(records); r++ {
		row := records[r]
		dateStr := cell(row, indices.DateIdx)
		parsedDate, isValidDate := ParseDate(dateStr)
		if !isValidDate {
			// Excel stores dates as day counts when a cell isn't formatted as text
			if parsedDate, isValidDate = excelSerialDate(dateStr); !isValidDate {
				continue // metadata or subtotal line
			}
		}

		desc := strings.Join(strings.Fields(cell(row, indices.PartyIdx)+" "+cell(row, indices.DescIdx)), " ")

		var debit, credit float64
		if indices.DebitIdx >= 0 || indices.CreditIdx >= 0 {
			debit = math.Abs(ParseAmount(cell(row, indices.DebitIdx)))
			credit = math.Abs(ParseAmount(cell(row, indices.CreditIdx)))
		}
		if debit == 0 && credit == 0 && indices.AmountIdx >= 0 {
			raw := cell(row, indices.AmountIdx)
			kind := strings.ToLower(cell(row, indices.TypeIdx))
			if kind == "" && strings.HasPrefix(raw, "+") {
				kind = "credit"
			}
			debit, credit = splitSignedAmount(ParseAmount(raw), kind)
		}

		sheet.Txs = append(sheet.Txs, models.RawTransaction{
			Date:        dateStr,
			ParsedDate:  parsedDate,
			Description: desc,
			Debit:       debit,
			Credit:      credit,
			Balance:     ParseAmount(cell(row, indices.BalanceIdx)),
			Reference:   cell(row, indices.RefIdx),
			RawLine:     strings.Join(row, " | "),
			Bank:        sheet.Bank,
			Currency:    currency,
		})
	}

	// One amount column with nothing saying which way each row went: the running balance decides.
	if indices.DebitIdx < 0 && indices.CreditIdx < 0 && indices.TypeIdx < 0 {
		reconcileBalances(sheet.Txs, 0, false)
	}
	if sheet.Bank == BankOPay {
		fillCardFundedTransfers(sheet.Txs)
	}
	if indices.HeaderRowFound {
		sheet.Check = CheckTotals(sheet.Txs, statedTotals(recordRows(records[:indices.HeaderRowIdx])))
	}
	return sheet
}

// headingText joins the rows above the table into text the bank and holder detectors can read.
func headingText(records [][]string) string {
	var b strings.Builder
	for _, row := range records {
		b.WriteString(strings.Join(row, ", "))
		b.WriteByte('\n')
		if b.Len() > 8192 {
			break
		}
	}
	return b.String()
}

// recordRows lays spreadsheet rows out as table rows, one column per 100 units, so the label and
// value finders written for PDFs work on them unchanged.
func recordRows(records [][]string) []tableRow {
	rows := make([]tableRow, 0, len(records))
	for i, rec := range records {
		row := tableRow{y: -float64(i)}
		for j, s := range rec {
			if strings.TrimSpace(s) != "" {
				row.cells = append(row.cells, tableCell{x: float64(j) * 100, y: row.y, w: 50, size: 10, s: s})
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// excelSerialDate reads a date stored as Excel's day count (45000.5 = 15 Mar 2023, midday).
func excelSerialDate(s string) (t time.Time, ok bool) {
	v := ParseAmount(s)
	if v < 32874 || v > 73050 || strings.ContainsAny(s, "-/: ") { // 1990 to 2099
		return t, false
	}
	t, err := excelize.ExcelDateToTime(v, false)
	return t, err == nil
}

func detectHeaderIndices(records [][]string) columnIndices {
	none := columnIndices{DateIdx: -1, DescIdx: -1, PartyIdx: -1, DebitIdx: -1, CreditIdx: -1,
		AmountIdx: -1, BalanceIdx: -1, RefIdx: -1, TypeIdx: -1}

	for rowIdx, row := range records {
		if rowIdx > 60 {
			break // a header this far down is a data row that happens to contain header words
		}
		idx := none
		valueDate := -1
		set := func(field *int, col int) {
			if *field == -1 {
				*field = col
			}
		}
		for colIdx, cell := range row {
			lower := strings.ToLower(strings.TrimSpace(cell))
			kind := headerKind(cell)
			switch {
			case kind == colTransDate:
				set(&idx.DateIdx, colIdx)
			case kind == colValueDate:
				set(&valueDate, colIdx)
			case kind == colRemarks || strings.Contains(lower, "memo"):
				set(&idx.DescIdx, colIdx)
			case kind == colParty:
				set(&idx.PartyIdx, colIdx)
			case kind == colDebit:
				set(&idx.DebitIdx, colIdx)
			case kind == colCredit:
				set(&idx.CreditIdx, colIdx)
			case kind == colAmount || lower == "value":
				set(&idx.AmountIdx, colIdx)
			case kind == colBalance:
				set(&idx.BalanceIdx, colIdx)
			case kind == colReference || lower == "stan" || lower == "id":
				set(&idx.RefIdx, colIdx)
			case kind == colType || lower == "txn type" || lower == "direction":
				set(&idx.TypeIdx, colIdx)
			}
		}
		if idx.DateIdx == -1 {
			idx.DateIdx = valueDate
		}

		// A header has a date, a description, and an amount or debit/credit
		money := idx.AmountIdx != -1 || idx.DebitIdx != -1 || idx.CreditIdx != -1
		if idx.DateIdx != -1 && (idx.DescIdx != -1 || idx.PartyIdx != -1) && money {
			idx.HeaderRowFound = true
			idx.HeaderRowIdx = rowIdx
			return idx
		}
	}

	// No header row: assume date, description, debit, credit, balance
	fallback := none
	fallback.DateIdx, fallback.DescIdx, fallback.DebitIdx, fallback.CreditIdx, fallback.BalanceIdx = 0, 1, 2, 3, 4
	return fallback
}
