package parser

import (
	"bytes"
	"encoding/csv"
	"io"
	"math"
	"strings"

	"azaman-engine/internal/models"
	"github.com/xuri/excelize/v2"
)

type columnIndices struct {
	DateIdx        int
	DescIdx        int
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
func ParseCSV(r io.Reader, bankName string) ([]models.RawTransaction, error) {
	// Read full content to determine separator (comma, semicolon, or tab)
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, r); err != nil {
		return nil, err
	}
	content := buf.Bytes()
	if len(content) == 0 {
		return []models.RawTransaction{}, nil
	}

	defaultCurrency := DetectCurrency(string(content[:int(math.Min(float64(len(content)), 2048))]))
	if bankName == BankGeegpay {
		defaultCurrency = "USD"
	}

	// Detect delimiter
	sample := string(content[:int(math.Min(float64(len(content)), 1024))])
	delimiter := ','
	if strings.Count(sample, ";") > strings.Count(sample, ",") {
		delimiter = ';'
	} else if strings.Count(sample, "\t") > strings.Count(sample, ",") {
		delimiter = '\t'
	}

	reader := csv.NewReader(bytes.NewReader(content))
	reader.Comma = delimiter
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1 // Allow variable number of fields

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	return processRecords(records, bankName, defaultCurrency)
}

// ParseXLSX reads transactions from an Excel workbook entirely in-memory.
func ParseXLSX(data []byte, bankName string) ([]models.RawTransaction, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return []models.RawTransaction{}, nil
	}

	// Read rows from the first sheet
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}

	defaultCurrency := "NGN"
	if bankName == BankGeegpay {
		defaultCurrency = "USD"
	}

	return processRecords(rows, bankName, defaultCurrency)
}

func processRecords(records [][]string, bankName, defaultCurrency string) ([]models.RawTransaction, error) {
	indices := detectHeaderIndices(records)
	var rawTransactions []models.RawTransaction

	startRow := 0
	if indices.HeaderRowFound {
		startRow = indices.HeaderRowIdx + 1
	}

	for r := startRow; r < len(records); r++ {
		row := records[r]
		if len(row) == 0 {
			continue
		}

		// Ensure row has sufficient columns
		maxIdx := max(indices.DateIdx, indices.DescIdx, indices.DebitIdx, indices.CreditIdx, indices.AmountIdx, indices.BalanceIdx)
		if len(row) <= maxIdx {
			continue
		}

		dateStr := strings.TrimSpace(row[indices.DateIdx])
		parsedDate, isValidDate := ParseDate(dateStr)
		if !isValidDate {
			// Skip metadata or subtotal lines that don't start with a valid date
			continue
		}

		desc := ""
		if indices.DescIdx >= 0 && indices.DescIdx < len(row) {
			desc = strings.TrimSpace(row[indices.DescIdx])
		}

		var debit, credit float64

		if indices.DebitIdx >= 0 && indices.CreditIdx >= 0 &&
			indices.DebitIdx < len(row) && indices.CreditIdx < len(row) {
			// Separate debit and credit columns
			debit = math.Abs(ParseAmount(row[indices.DebitIdx]))
			credit = math.Abs(ParseAmount(row[indices.CreditIdx]))
		} else if indices.AmountIdx >= 0 && indices.AmountIdx < len(row) {
			// Single amount column
			amt := ParseAmount(row[indices.AmountIdx])
			typeStr := ""
			if indices.TypeIdx >= 0 && indices.TypeIdx < len(row) {
				typeStr = strings.ToUpper(strings.TrimSpace(row[indices.TypeIdx]))
			}

			if strings.Contains(typeStr, "DR") || strings.Contains(typeStr, "DEBIT") || strings.Contains(typeStr, "OUT") {
				debit = math.Abs(amt)
			} else if strings.Contains(typeStr, "CR") || strings.Contains(typeStr, "CREDIT") || strings.Contains(typeStr, "IN") {
				credit = math.Abs(amt)
			} else {
				// Infer from sign
				if amt < 0 {
					debit = math.Abs(amt)
				} else {
					credit = amt
				}
			}
		}

		balance := 0.0
		if indices.BalanceIdx >= 0 && indices.BalanceIdx < len(row) {
			balance = ParseAmount(row[indices.BalanceIdx])
		}

		ref := ""
		if indices.RefIdx >= 0 && indices.RefIdx < len(row) {
			ref = strings.TrimSpace(row[indices.RefIdx])
		}

		rawTransactions = append(rawTransactions, models.RawTransaction{
			Date:        dateStr,
			ParsedDate:  parsedDate,
			Description: desc,
			Debit:       debit,
			Credit:      credit,
			Balance:     balance,
			Reference:   ref,
			RawLine:     strings.Join(row, " | "),
			Bank:        bankName,
			Currency:    defaultCurrency,
		})
	}

	return rawTransactions, nil
}

func detectHeaderIndices(records [][]string) columnIndices {
	indices := columnIndices{
		DateIdx:    -1,
		DescIdx:    -1,
		DebitIdx:   -1,
		CreditIdx:  -1,
		AmountIdx:  -1,
		BalanceIdx: -1,
		RefIdx:     -1,
		TypeIdx:    -1,
	}

	for rowIdx, row := range records {
		dateCol := -1
		descCol := -1
		debitCol := -1
		creditCol := -1
		amountCol := -1
		balanceCol := -1
		refCol := -1
		typeCol := -1

		for colIdx, cell := range row {
			header := strings.ToLower(strings.TrimSpace(cell))

			if strings.Contains(header, "date") || header == "time" || header == "trans date" || header == "txn date" || header == "value date" {
				if dateCol == -1 {
					dateCol = colIdx
				}
			} else if strings.Contains(header, "desc") || strings.Contains(header, "narration") ||
				strings.Contains(header, "remark") || strings.Contains(header, "detail") ||
				strings.Contains(header, "particular") || strings.Contains(header, "memo") {
				descCol = colIdx
			} else if strings.Contains(header, "debit") || strings.Contains(header, "dr") ||
				strings.Contains(header, "withdrawal") || strings.Contains(header, "paid out") ||
				strings.Contains(header, "money out") {
				debitCol = colIdx
			} else if strings.Contains(header, "credit") || strings.Contains(header, "cr") ||
				strings.Contains(header, "deposit") || strings.Contains(header, "paid in") ||
				strings.Contains(header, "money in") {
				creditCol = colIdx
			} else if header == "amount" || header == "txn amount" || header == "value" {
				amountCol = colIdx
			} else if strings.Contains(header, "balance") {
				balanceCol = colIdx
			} else if strings.Contains(header, "ref") || strings.Contains(header, "stan") || strings.Contains(header, "id") {
				refCol = colIdx
			} else if header == "type" || header == "txn type" || header == "direction" {
				typeCol = colIdx
			}
		}

		// A header is valid if it has at least a date and description, plus an amount or debit/credit
		if dateCol != -1 && descCol != -1 && (amountCol != -1 || (debitCol != -1 || creditCol != -1)) {
			indices.DateIdx = dateCol
			indices.DescIdx = descCol
			indices.DebitIdx = debitCol
			indices.CreditIdx = creditCol
			indices.AmountIdx = amountCol
			indices.BalanceIdx = balanceCol
			indices.RefIdx = refCol
			indices.TypeIdx = typeCol
			indices.HeaderRowFound = true
			indices.HeaderRowIdx = rowIdx
			return indices
		}
	}

	// Fallback to standard 0-indexed positions if no explicit header was matched
	indices.DateIdx = 0
	indices.DescIdx = 1
	indices.DebitIdx = 2
	indices.CreditIdx = 3
	indices.BalanceIdx = 4
	indices.HeaderRowFound = false
	return indices
}

func max(nums ...int) int {
	m := nums[0]
	for _, n := range nums[1:] {
		if n > m {
			m = n
		}
	}
	return m
}
