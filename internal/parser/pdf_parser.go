package parser

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"regexp"
	"sort"
	"strings"

	"azaman-engine/internal/models"
	"github.com/ledongthuc/pdf"
)

var (
	// Regex to identify if a token looks like a date
	dateTokenRe = regexp.MustCompile(`^(?:\d{1,2}[-/][A-Za-z]{3}[-/]\d{2,4}|\d{1,2}[-/]\d{1,2}[-/]\d{2,4}|\d{4}[-/]\d{1,2}[-/]\d{1,2}|\d{1,2}\s+[A-Za-z]{3}\s+\d{2,4})$`)
	// Regex to check if a token looks like a currency amount
	amountTokenRe = regexp.MustCompile(`^[₦$€£]?-?\(?\d{1,3}(?:,\d{3})*(?:\.\d{1,2})?\)?(?:CR|DR)?$`)
)

// ParsePDF parses an encrypted or unencrypted bank statement PDF entirely in RAM.
func ParsePDF(data []byte, password string, bankName string) (txs []models.RawTransaction, detectedBank string, err error) {
	detectedBank = bankName
	if len(data) == 0 {
		return nil, detectedBank, fmt.Errorf("empty PDF byte buffer")
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("PDF extraction error: %v", r)
			log.Printf("[PDF-DIAG] recovered panic: %v", r)
		}
	}()

	readerAt := bytes.NewReader(data)
	size := int64(len(data))

	var pdfReader *pdf.Reader

	if password != "" {
		pdfReader, err = pdf.NewReaderEncrypted(readerAt, size, func() string {
			return password
		})
	} else {
		pdfReader, err = pdf.NewReader(readerAt, size)
		if err != nil && (strings.Contains(err.Error(), "password") || strings.Contains(err.Error(), "encrypted")) {
			// Try with blank password or prompt
			pdfReader, err = pdf.NewReaderEncrypted(readerAt, size, func() string {
				return ""
			})
		}
	}

	if err != nil {
		return nil, detectedBank, fmt.Errorf("failed to open PDF in memory: %w", err)
	}

	numPages := pdfReader.NumPage()
	var allLines []string
	var sampleText strings.Builder
	var nullPages, rowPages, plainPages, contentPages int
	hintBank := bankName
	var opayRows int
	// Diagnostic summary: counts and bank names only, never statement text.
	defer func() {
		log.Printf("[PDF-DIAG] pages=%d null=%d byRow=%d content=%d plain=%d lines=%d hint=%q detected=%q opayRows=%d finalRows=%d err=%v",
			numPages, nullPages, rowPages, contentPages, plainPages, len(allLines), hintBank, detectedBank, opayRows, len(txs), err)
		if len(txs) == 0 {
			for i := 0; i < len(allLines) && i < 25; i++ {
				log.Printf("[PDF-DIAG] line %02d shape: %s", i, lineShape(allLines[i]))
			}
		}
	}()

	for p := 1; p <= numPages; p++ {
		page := pdfReader.Page(p)
		if page.V.IsNull() {
			nullPages++
			continue
		}

		// Try extracting structured rows first
		rows, rowErr := page.GetTextByRow()

		// GetTextByRow ignores Td/cm positioning; if the page collapsed into a couple of rows,
		// rebuild lines from the fully interpreted content instead.
		if rowErr == nil && len(rows) <= 2 {
			lines, cErr := contentRowLines(page)
			if cErr != nil {
				log.Printf("[PDF-DIAG] page %d: %v", p, cErr)
			}
			if len(lines) > len(rows) {
				contentPages++
				for _, joined := range lines {
					allLines = append(allLines, joined)
					if sampleText.Len() < 8192 {
						sampleText.WriteString(joined + "\n")
					}
				}
				continue
			}
		}

		if rowErr == nil && len(rows) > 0 {
			rowPages++
			// Sort rows top-to-bottom
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].Position > rows[j].Position
			})

			for _, row := range rows {
				var lineParts []string
				for _, cell := range row.Content {
					trimmed := strings.TrimSpace(cell.S)
					if trimmed != "" {
						lineParts = append(lineParts, trimmed)
					}
				}
				if len(lineParts) > 0 {
					joined := strings.Join(lineParts, " ")
					allLines = append(allLines, joined)
					if sampleText.Len() < 8192 {
						sampleText.WriteString(joined + "\n")
					}
				}
			}
		} else {
			// Fallback to plain text
			plainPages++
			text, textErr := page.GetPlainText(nil)
			if textErr == nil {
				lines := strings.Split(text, "\n")
				for _, l := range lines {
					trimmed := strings.TrimSpace(l)
					if trimmed != "" {
						allLines = append(allLines, trimmed)
						if sampleText.Len() < 8192 {
							sampleText.WriteString(trimmed + "\n")
						}
					}
				}
			}
		}
	}

	if len(allLines) == 0 {
		return nil, detectedBank, fmt.Errorf("this PDF contains no selectable text (it may be a scanned image or empty). Please upload an electronic PDF statement or CSV/Excel file")
	}

	// Auto-detect bank from extracted text if not already determined
	if detectedBank == "" || detectedBank == BankGeneric {
		detectedBank = DetectBank(sampleText.String())
	}

	defaultCurrency := DetectCurrency(sampleText.String())
	if detectedBank == BankGeegpay {
		defaultCurrency = "USD"
	}

	// Use specialized column-aware parser for OPay statements if detected
	if detectedBank == BankOPay {
		opayTxs := parseOPayPDFRows(pdfReader, detectedBank, defaultCurrency)
		opayRows = len(opayTxs)
		if len(opayTxs) > 0 {
			return opayTxs, detectedBank, nil
		}
	}

	transactions := parseLinesToTransactions(allLines, detectedBank, defaultCurrency)
	if len(transactions) == 0 {
		return nil, detectedBank, fmt.Errorf("no transaction table rows could be identified in this PDF. Please verify this is an account statement with tabular dates and amounts")
	}

	return transactions, detectedBank, nil
}

// isPDFHeaderNoise checks whether a cell represents table header text rather than transaction data.
func isPDFHeaderNoise(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "trans. time") ||
		strings.Contains(lower, "value date") ||
		strings.Contains(lower, "description") ||
		strings.Contains(lower, "debit") ||
		strings.Contains(lower, "credit") ||
		strings.Contains(lower, "balance after") ||
		strings.Contains(lower, "account statement") ||
		strings.Contains(lower, "opening balance") ||
		strings.Contains(lower, "closing balance") ||
		strings.Contains(lower, "total debit") ||
		strings.Contains(lower, "total credit") ||
		strings.Contains(lower, "period:") ||
		strings.Contains(lower, "generated on") ||
		strings.Contains(lower, "wallet account") ||
		strings.Contains(lower, "channel") ||
		strings.Contains(lower, "transaction reference") ||
		strings.Contains(lower, "page ")
}

// parseOPayPDFRows extracts transactions from OPay's multi-column tabular PDF layout.
func parseOPayPDFRows(pdfReader *pdf.Reader, detectedBank, currency string) []models.RawTransaction {
	numPages := pdfReader.NumPage()
	var results []models.RawTransaction
	months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

	for p := 1; p <= numPages; p++ {
		page := pdfReader.Page(p)
		if page.V.IsNull() {
			continue
		}

		rows, err := page.GetTextByRow()
		if err != nil || len(rows) == 0 {
			continue
		}

		sort.Slice(rows, func(i, j int) bool {
			return rows[i].Position > rows[j].Position
		})

		var txRows []int
		for i, r := range rows {
			if (p == 1 && r.Position > 450) || (p > 1 && r.Position > 600) {
				continue
			}
			for _, cell := range r.Content {
				if cell.X >= 60 && cell.X <= 85 {
					txt := strings.TrimSpace(cell.S)
					if len(txt) >= 10 {
						hasMonth := false
						for _, m := range months {
							if strings.Contains(txt, m) {
								hasMonth = true
								break
							}
						}
						if hasMonth {
							txRows = append(txRows, i)
							break
						}
					}
				}
			}
		}

		for idx, rowIdx := range txRows {
			datePos := float64(rows[rowIdx].Position)
			var tx models.RawTransaction
			tx.Bank = detectedBank
			tx.Currency = currency

			upperY := 100000.0
			if idx > 0 {
				prevPos := float64(rows[txRows[idx-1]].Position)
				upperY = (datePos + prevPos) / 2.0
			}
			lowerY := -1.0
			if idx+1 < len(txRows) {
				nextPos := float64(rows[txRows[idx+1]].Position)
				lowerY = (datePos + nextPos) / 2.0
			}

			var descLines []string

			for _, r := range rows {
				rPos := float64(r.Position)
				if rPos > upperY || rPos <= lowerY {
					continue
				}
				if (p == 1 && r.Position > 450) || (p > 1 && r.Position > 600) {
					continue
				}

				for _, cell := range r.Content {
					txt := strings.TrimSpace(cell.S)
					if isPDFHeaderNoise(txt) || txt == "" {
						continue
					}

					if cell.X >= 60 && cell.X <= 85 && tx.Date == "" {
						tx.Date = txt
					} else if cell.X >= 160 && cell.X <= 280 {
						descLines = append(descLines, txt)
					} else if cell.X >= 295 && cell.X <= 335 && tx.Debit == 0 {
						tx.Debit = ParseAmount(txt)
					} else if cell.X >= 335 && cell.X <= 375 && tx.Credit == 0 {
						tx.Credit = ParseAmount(txt)
					} else if cell.X >= 375 && cell.X <= 415 && tx.Balance == 0 {
						tx.Balance = ParseAmount(txt)
					} else if cell.X >= 460 && tx.Reference == "" {
						tx.Reference = txt
					}
				}
			}

			tx.Description = strings.Join(descLines, " ")
			tx.ParsedDate, _ = ParseDate(tx.Date)
			tx.RawLine = fmt.Sprintf("%s | %s | Debit: %.2f | Credit: %.2f | Bal: %.2f",
				tx.Date, tx.Description, tx.Debit, tx.Credit, tx.Balance)

			if tx.Debit > 0 || tx.Credit > 0 || tx.Description != "" {
				results = append(results, tx)
			}
		}
	}

	return results
}

// parseLinesToTransactions groups consecutive text lines into discrete transactions.
func parseLinesToTransactions(lines []string, bankName, currency string) []models.RawTransaction {
	var results []models.RawTransaction
	var currentTx *models.RawTransaction

	flushCurrent := func() {
		if currentTx != nil && (currentTx.Debit > 0 || currentTx.Credit > 0 || currentTx.Description != "") {
			results = append(results, *currentTx)
		}
		currentTx = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}

		// Check if first token or second token is a valid date
		firstToken := fields[0]
		parsedDate, isDate := ParseDate(firstToken)

		// Handle cases where first token is a row number like "1" or "01" and second is date
		dateOffset := 0
		if !isDate && len(fields) > 1 {
			if d, ok := ParseDate(fields[1]); ok {
				parsedDate = d
				isDate = true
				dateOffset = 1
			}
		}

		// Also check 3-token date format: "15 Jan 2024"
		if !isDate && len(fields) >= 3 {
			threeTokenDate := strings.Join(fields[0:3], " ")
			if d, ok := ParseDate(threeTokenDate); ok {
				parsedDate = d
				isDate = true
				dateOffset = 2
			}
		}

		if isDate {
			// Found new transaction header line
			flushCurrent()

			dateStr := firstToken
			if dateOffset == 1 && len(fields) > 1 {
				dateStr = fields[1]
			} else if dateOffset == 2 && len(fields) >= 3 {
				dateStr = strings.Join(fields[0:3], " ")
			}

			tx := models.RawTransaction{
				Date:       dateStr,
				ParsedDate: parsedDate,
				Bank:       bankName,
				Currency:   currency,
				RawLine:    line,
			}

			// Process remaining tokens
			remaining := fields[dateOffset+1:]
			tx.Description, tx.Debit, tx.Credit, tx.Balance, tx.Reference = extractFieldsFromTokens(remaining)
			currentTx = &tx
		} else {
			// Multiline narration continuation or footer/header noise
			if currentTx != nil {
				lower := strings.ToLower(trimmed)
				// Skip obvious page headers/footers
				if strings.Contains(lower, "page ") || strings.Contains(lower, "generated on") ||
					strings.Contains(lower, "trans date") || strings.Contains(lower, "statement of account") {
					continue
				}

				// Check if this continuation line has amounts if currentTx didn't have them yet
				if currentTx.Debit == 0 && currentTx.Credit == 0 {
					_, d, c, b, r := extractFieldsFromTokens(fields)
					if d > 0 || c > 0 {
						currentTx.Debit = d
						currentTx.Credit = c
						if b != 0 {
							currentTx.Balance = b
						}
						if r != "" {
							currentTx.Reference = r
						}
					}
				}

				// Append to current description
				if currentTx.Description == "" {
					currentTx.Description = trimmed
				} else {
					currentTx.Description += " " + trimmed
				}
				currentTx.RawLine += " \n " + trimmed
			}
		}
	}

	flushCurrent()
	return results
}

// extractFieldsFromTokens separates narration text from numeric amount/balance columns.
func extractFieldsFromTokens(tokens []string) (desc string, debit float64, credit float64, balance float64, ref string) {
	if len(tokens) == 0 {
		return "", 0, 0, 0, ""
	}

	type numItem struct {
		val float64
		idx int
	}

	var numericTokens []numItem
	var textTokens []string

	for i, t := range tokens {
		// Clean and parse
		cleanedT := strings.Trim(t, "(),")
		if amt := ParseAmount(cleanedT); amt != 0 || t == "0.00" || t == "0" || t == "-0.00" {
			// Verify it looks like an amount rather than a word
			if amountTokenRe.MatchString(t) || t == "0.00" || t == "0" {
				numericTokens = append(numericTokens, numItem{val: amt, idx: i})
				continue
			}
		}
		textTokens = append(textTokens, t)
	}

	desc = strings.Join(textTokens, " ")

	// If we have at least 2 numbers (amount and balance) or 3 (debit, credit, balance)
	if len(numericTokens) >= 3 {
		// Typically: Debit, Credit, Balance
		debit = math.Abs(numericTokens[0].val)
		credit = math.Abs(numericTokens[1].val)
		balance = numericTokens[2].val
	} else if len(numericTokens) == 2 {
		amt := numericTokens[0].val
		balance = numericTokens[1].val
		if amt < 0 {
			debit = math.Abs(amt)
		} else {
			credit = amt
		}
	} else if len(numericTokens) == 1 {
		amt := numericTokens[0].val
		if amt < 0 {
			debit = math.Abs(amt)
		} else {
			credit = amt
		}
	}

	return desc, debit, credit, balance, ref
}
