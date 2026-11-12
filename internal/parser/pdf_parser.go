package parser

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

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
func ParsePDF(data []byte, password string, bankName string) (txs []models.RawTransaction, detectedBank string, check models.StatementCheck, err error) {
	detectedBank = bankName
	if len(data) == 0 {
		return nil, detectedBank, check, fmt.Errorf("empty PDF byte buffer")
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
		// The library keeps calling the callback until it returns "", so supply the
		// password once; otherwise a wrong password loops forever.
		supplied := false
		pdfReader, err = pdf.NewReaderEncrypted(readerAt, size, func() string {
			if supplied {
				return ""
			}
			supplied = true
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
		return nil, detectedBank, check, fmt.Errorf("failed to open PDF in memory: %w", err)
	}

	numPages := pdfReader.NumPage()
	var allLines []string
	var sampleText strings.Builder
	var nullPages, rowPages, plainPages, contentPages int
	hintBank := bankName
	var opayRows, tableRows int
	// Diagnostic summary: counts and bank names only, never statement text.
	defer func() {
		log.Printf("[PDF-DIAG] pages=%d null=%d byRow=%d content=%d plain=%d lines=%d hint=%q detected=%q opayRows=%d tableRows=%d finalRows=%d err=%v",
			numPages, nullPages, rowPages, contentPages, plainPages, len(allLines), hintBank, detectedBank, opayRows, tableRows, len(txs), err)
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
		return nil, detectedBank, check, fmt.Errorf("this PDF contains no selectable text (it may be a scanned image or empty). Please upload an electronic PDF statement or CSV/Excel file")
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
		opayTxs, stated := parseOPayPDFRows(pdfReader, detectedBank, defaultCurrency)
		opayRows = len(opayTxs)
		if len(opayTxs) > 0 {
			return opayTxs, detectedBank, CheckTotals(opayTxs, stated), nil
		}
	}

	// Column-aware table parser (header-anchored, balance-reconciled) for other banks
	var pageRunList [][]pdf.Text
	for p := 1; p <= numPages; p++ {
		page := pdfReader.Page(p)
		if page.V.IsNull() {
			continue
		}
		runs, rErr := pageRuns(page)
		if rErr != nil {
			log.Printf("[PDF-DIAG] page %d: %v", p, rErr)
			continue
		}
		pageRunList = append(pageRunList, runs)
	}
	if tableTxs, stated := parseTablePDF(pageRunList, detectedBank, defaultCurrency); len(tableTxs) > 0 {
		tableRows = len(tableTxs)
		return tableTxs, detectedBank, CheckTotals(tableTxs, stated), nil
	}

	transactions := parseLinesToTransactions(allLines, detectedBank, defaultCurrency)
	if len(transactions) == 0 {
		return nil, detectedBank, check, fmt.Errorf("no transaction table rows could be identified in this PDF. Please verify this is an account statement with tabular dates and amounts")
	}

	return transactions, detectedBank, check, nil
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

// parseOPayPDFRows extracts transactions from OPay's tabular PDF layout. Column positions come
// from the "Trans. Time ... Transaction Reference" header, so each value lands in its column.
// Descriptions and references wrap above and below the date line, so each transaction owns the
// rows between the midpoints to its neighbours. Only the first account section (the wallet) is
// read: the OWealth "Savings Account" section that follows has its own layout, and its movements
// already appear in the wallet as "Auto-save to OWealth" / "OWealth Withdrawal".
func parseOPayPDFRows(pdfReader *pdf.Reader, detectedBank, currency string) ([]models.RawTransaction, *models.StatementCheck) {
	var results []models.RawTransaction
	var stated *models.StatementCheck
	var cols []opayColumn
	var carryDesc, carryRefs []string // first lines of a row split across a page break
	sections := 0

	for p := 1; p <= pdfReader.NumPage(); p++ {
		page := pdfReader.Page(p)
		if page.V.IsNull() {
			continue
		}
		rows, err := page.GetTextByRow()
		if err != nil || len(rows) == 0 {
			continue
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].Position > rows[j].Position })

		var body pdf.Rows
		for i, r := range rows {
			text := rowString(r)
			if strings.Contains(text, "Opening Balance") {
				sections++
				if sections > 1 {
					break // next account section
				}
			}
			if stated == nil && strings.Contains(text, "Total Debit") && i+3 < len(rows) {
				stated = opayStatedTotals(rows[i : i+4])
			}
			if strings.Contains(text, "Trans. Time") {
				cols = opayColumns(rows, i)
				body = body[:0]
				continue
			}
			if cols != nil {
				body = append(body, r)
			}
		}
		if cols == nil {
			continue // cover rows before the table header
		}

		var dateRows []int
		for i, r := range body {
			for _, c := range r.Content {
				if opayColumnAt(cols, c.X) == colTransDate {
					if _, ok := ParseDate(strings.TrimSpace(c.S)); ok {
						dateRows = append(dateRows, i)
					}
					break
				}
			}
		}

		// A wrapped reference is centred on its date line, so it reaches as far below the date as
		// above. That matters at page edges, where OPay splits a row in two: lines left under the
		// last date belong to the first transaction of the next page, and lines above the first
		// date beyond its own reach finish the previous page's last one.
		reach := func(ri int, from, to float64, above bool) float64 {
			y, far := float64(body[ri].Position), 0.0
			for _, r := range body {
				pos := float64(r.Position)
				if pos > to || pos <= from || (above && pos <= y) || (!above && pos >= y) {
					continue
				}
				for _, c := range r.Content {
					if opayColumnAt(cols, c.X) == colReference && strings.TrimSpace(c.S) != "" {
						far = math.Max(far, math.Abs(pos-y))
					}
				}
			}
			return far
		}
		collectRefs := func(from, to float64) (refs []string) {
			for _, r := range body {
				if pos := float64(r.Position); pos > to || pos <= from {
					continue
				}
				for _, c := range r.Content {
					if opayColumnAt(cols, c.X) == colReference && strings.TrimSpace(c.S) != "" {
						refs = append(refs, strings.TrimSpace(c.S))
					}
				}
			}
			return refs
		}

		dateYs := make([]float64, len(dateRows))
		for k, ri := range dateRows {
			dateYs[k] = float64(body[ri].Position)
		}
		descOf, leadDesc, nextDesc := opayDescriptions(body, cols, dateYs)
		if len(leadDesc) > 0 && len(results) > 0 {
			last := &results[len(results)-1]
			last.Description = joinWrapped(append([]string{last.Description + " "}, leadDesc...))
			last.RawLine = fmt.Sprintf("%s | %s | Debit: %.2f | Credit: %.2f | Bal: %.2f",
				last.Date, last.Description, last.Debit, last.Credit, last.Balance)
		} else if len(leadDesc) > 0 && len(dateYs) > 0 {
			descOf[0] = append(leadDesc, descOf[0]...)
		}

		var nextRefs []string // reference lines under this page's last transaction, owed to the next
		for k, ri := range dateRows {
			y := float64(body[ri].Position)
			upper, lower := math.Inf(1), math.Inf(-1)
			if k > 0 {
				upper = (y + float64(body[dateRows[k-1]].Position)) / 2
			}
			if k+1 < len(dateRows) {
				lower = (y + float64(body[dateRows[k+1]].Position)) / 2
			}
			if k == 0 && len(results) > 0 {
				upper = y + reach(ri, lower, y, false) + opayLineSlack
				results[len(results)-1].Reference += strings.Join(collectRefs(upper, math.Inf(1)), "")
			}
			if k == len(dateRows)-1 {
				lower = y - reach(ri, y, upper, true) - opayLineSlack
				nextRefs = collectRefs(y-opayRowPitch, lower)
			}

			tx := models.RawTransaction{Bank: detectedBank, Currency: currency}
			desc := append(carryDesc, descOf[k]...)
			refs := carryRefs
			carryDesc, carryRefs = nil, nil
			for _, r := range body {
				if pos := float64(r.Position); pos > upper || pos <= lower {
					continue
				}
				for _, c := range r.Content {
					txt := strings.TrimSpace(c.S)
					if txt == "" {
						continue
					}
					switch opayColumnAt(cols, c.X) {
					case colTransDate:
						if tx.Date == "" {
							tx.Date = txt
						}
					case colDebit:
						tx.Debit = ParseAmount(txt)
					case colCredit:
						tx.Credit = ParseAmount(txt)
					case colBalance:
						tx.Balance = ParseAmount(txt)
					case colReference:
						refs = append(refs, txt)
					}
				}
			}

			tx.Description = joinWrapped(desc)
			tx.Reference = strings.Join(refs, "")
			tx.ParsedDate, _ = ParseDate(tx.Date)
			tx.RawLine = fmt.Sprintf("%s | %s | Debit: %.2f | Credit: %.2f | Bal: %.2f",
				tx.Date, tx.Description, tx.Debit, tx.Credit, tx.Balance)
			results = append(results, tx)
		}
		carryDesc, carryRefs = nextDesc, nextRefs
		if sections > 1 {
			break
		}
	}

	fillCardFundedTransfers(results)
	return results, stated
}

// opayStatedTotals reads the summary block under the wallet heading:
//
//	Opening Balance   Total Debit      Debit Count
//	₦ 0.00            ₦ 20,224,221.88  4404
//	Closing Balance   Total Credit     Credit Count
//	₦ 0.00            ₦ 20,224,221.88  3755
//
// Values sit under their labels, so each is the amount on the next row nearest the label's x.
func opayStatedTotals(rows pdf.Rows) *models.StatementCheck {
	valueUnder := func(label string) (float64, bool) {
		for i := 0; i+1 < len(rows); i++ {
			for _, c := range rows[i].Content {
				if !strings.HasPrefix(strings.TrimSpace(c.S), label) {
					continue
				}
				best, found := 0.0, false
				dist := math.Inf(1)
				for _, v := range rows[i+1].Content {
					s := strings.TrimSpace(v.S)
					if s == "" || s == "₦" {
						continue
					}
					if d := math.Abs(v.X - c.X); d < dist && d < 40 {
						dist, best, found = d, ParseAmount(s), true
					}
				}
				return best, found
			}
		}
		return 0, false
	}
	debit, ok1 := valueUnder("Total Debit")
	credit, ok2 := valueUnder("Total Credit")
	nd, ok3 := valueUnder("Debit Count")
	nc, ok4 := valueUnder("Credit Count")
	if !(ok1 && ok2) {
		return nil
	}
	check := &models.StatementCheck{StatedDebit: debit, StatedCredit: credit}
	if ok3 && ok4 {
		check.StatedRows = int(nd + nc)
	}
	return check
}

// CheckTotals compares parsed transactions with the statement's printed totals. Amounts may differ
// by up to 0.5% (card-funded transfers carry the top-up amount, which can include a card fee).
func CheckTotals(txs []models.RawTransaction, stated *models.StatementCheck) models.StatementCheck {
	if stated == nil {
		return models.StatementCheck{}
	}
	check := *stated
	check.Checked = true
	check.ParsedRows = len(txs)
	for _, tx := range txs {
		check.ParsedDebit += tx.Debit
		check.ParsedCredit += tx.Credit
	}
	check.ParsedDebit = math.Round(check.ParsedDebit*100) / 100
	check.ParsedCredit = math.Round(check.ParsedCredit*100) / 100
	close := func(parsed, stated float64) bool {
		return math.Abs(parsed-stated) <= math.Max(1, stated*0.005)
	}
	check.OK = close(check.ParsedDebit, check.StatedDebit) && close(check.ParsedCredit, check.StatedCredit) &&
		(check.StatedRows == 0 || check.StatedRows == check.ParsedRows)
	return check
}

// fillCardFundedTransfers recovers amounts OPay leaves at 0.00: a transfer paid straight from a
// bank card shows no debit, and the card charge appears seconds later as an "Add Money-Bank Card"
// credit. That credit is the best available amount for the transfer (it can include a card fee).
func fillCardFundedTransfers(txs []models.RawTransaction) {
	for i := 0; i+1 < len(txs); i++ {
		tx, next := &txs[i], txs[i+1]
		if tx.Debit != 0 || tx.Credit != 0 || next.Credit <= 0 ||
			!strings.HasPrefix(strings.ToLower(next.Description), "add money") {
			continue
		}
		if gap := next.ParsedDate.Sub(tx.ParsedDate); gap < 0 || gap > 2*time.Minute {
			continue
		}
		tx.Debit = next.Credit
		tx.RawLine = fmt.Sprintf("%s | %s | Debit: %.2f | Credit: %.2f | Bal: %.2f",
			tx.Date, tx.Description, tx.Debit, tx.Credit, tx.Balance)
	}
}

const (
	opayLineSlack = 2.0  // tolerance when measuring how far a cell's lines reach
	opayRowPitch  = 45.0 // lines further below a page's last date are footer, not a split row
	opayCellGap   = 16.5 // between the 12-unit line spacing inside a cell and the 21 between cells
)

// opayDescriptions assigns the description column's lines to transactions. Lines of one cell are
// 12 units apart and cells are at least 21 apart, whatever their length, so a gap wider than
// opayCellGap starts a new cell. A cell belongs to the date line it surrounds. Cells with no date
// on this page are halves of rows split by a page break: above the first date they finish the
// previous page's last transaction (lead), below the last date they start the next page's first
// (next). dateYs are the date lines' positions, top to bottom.
func opayDescriptions(body pdf.Rows, cols []opayColumn, dateYs []float64) (perDate map[int][]string, lead, next []string) {
	type cell struct {
		top, bottom float64
		lines       []string
	}
	var cells []cell
	for _, r := range body {
		pos := float64(r.Position)
		var text string
		for _, c := range r.Content {
			if opayColumnAt(cols, c.X) == colRemarks {
				text += c.S
			}
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		if n := len(cells); n > 0 && cells[n-1].bottom-pos <= opayCellGap {
			cells[n-1].bottom = pos
			cells[n-1].lines = append(cells[n-1].lines, text)
			continue
		}
		cells = append(cells, cell{top: pos, bottom: pos, lines: []string{text}})
	}

	perDate = make(map[int][]string, len(dateYs))
	for _, c := range cells {
		owner, best := -1, math.Inf(1)
		for k, y := range dateYs {
			if y > c.top+opayCellGap/2 || y < c.bottom-opayCellGap/2 {
				continue
			}
			if d := math.Abs(y - (c.top+c.bottom)/2); d < best {
				owner, best = k, d
			}
		}
		switch {
		case owner >= 0:
			perDate[owner] = append(perDate[owner], c.lines...)
		case len(dateYs) == 0 || c.bottom > dateYs[0]:
			lead = append(lead, c.lines...)
		case c.top < dateYs[len(dateYs)-1]:
			if dateYs[len(dateYs)-1]-c.top <= opayRowPitch { // further down is footer text
				next = append(next, c.lines...)
			}
		default: // between two dates but around neither: give it to the nearer one
			for k, y := range dateYs {
				if d := math.Min(math.Abs(y-c.top), math.Abs(y-c.bottom)); d < best {
					owner, best = k, d
				}
			}
			perDate[owner] = append(perDate[owner], c.lines...)
		}
	}
	return perDate, lead, next
}

// joinWrapped rebuilds a cell from its wrapped lines. The PDF keeps the space at the end of a
// line that broke between words, so a line without one broke mid-word and joins with no space.
func joinWrapped(lines []string) string {
	return strings.Join(strings.Fields(strings.Join(lines, "")), " ")
}

type opayColumn struct {
	kind string
	x    float64 // left edge of the header label; values are left-aligned under it
}

// opayColumns reads column positions from the header row at index h. "Balance After (₦)" is
// drawn on the lines just above and below the header, so nearby rows are searched for it.
func opayColumns(rows pdf.Rows, h int) []opayColumn {
	var cols []opayColumn
	add := func(kind string, x float64) { cols = append(cols, opayColumn{kind, x}) }
	for i := h - 1; i <= h+1; i++ {
		if i < 0 || i >= len(rows) {
			continue
		}
		for _, c := range rows[i].Content {
			switch s := strings.TrimSpace(c.S); {
			case strings.HasPrefix(s, "Trans. Time"):
				add(colTransDate, c.X)
			case strings.HasPrefix(s, "Value Date"):
				add(colValueDate, c.X)
			case strings.HasPrefix(s, "Description"):
				add(colRemarks, c.X)
			case strings.HasPrefix(s, "Debit"):
				add(colDebit, c.X)
			case strings.HasPrefix(s, "Credit"):
				add(colCredit, c.X)
			case strings.HasPrefix(s, "Balance"):
				add(colBalance, c.X)
			case strings.HasPrefix(s, "Channel"):
				add(colBranch, c.X)
			case strings.HasPrefix(s, "Transaction Reference"):
				add(colReference, c.X)
			}
		}
	}
	sort.Slice(cols, func(i, j int) bool { return cols[i].x < cols[j].x })
	return cols
}

// opayColumnAt returns the column whose header starts at or just left of x.
func opayColumnAt(cols []opayColumn, x float64) string {
	kind := ""
	for _, c := range cols {
		if x+2 >= c.x {
			kind = c.kind
		}
	}
	return kind
}

func rowString(r *pdf.Row) string {
	var parts []string
	for _, c := range r.Content {
		parts = append(parts, c.S)
	}
	return strings.Join(parts, " ")
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
	// A lone amount column doesn't say which way the money went; the running balance does.
	reconcileBalances(results, 0, false)
	return results
}

// extractFieldsFromTokens separates narration text from the amount and balance columns. Money
// columns close a statement line and are printed with kobo ("1,500.00"), so the figures are the
// trailing tokens with a decimal part; whole numbers inside a narration (references, phone
// numbers, "2024") stay in the description. Lines with no decimal figure fall back to any number.
func extractFieldsFromTokens(tokens []string) (desc string, debit float64, credit float64, balance float64, ref string) {
	if len(tokens) == 0 {
		return "", 0, 0, 0, ""
	}

	isMoney := func(t string, needDecimal bool) bool {
		if !amountTokenRe.MatchString(t) {
			return false
		}
		return !needDecimal || moneyDecimalRe.MatchString(t)
	}
	figures := func(needDecimal bool) (vals []float64, raw []string, text []string) {
		end := len(tokens)
		for end > 0 && (isMoney(tokens[end-1], needDecimal) || isDirectionMark(tokens[end-1])) {
			end--
		}
		for _, t := range tokens[end:] {
			if isDirectionMark(t) { // "1,500.00 DR": the mark belongs to the figure before it
				if n := len(vals); n > 0 && strings.EqualFold(t, "DR") {
					vals[n-1] = -math.Abs(vals[n-1])
				}
				continue
			}
			vals = append(vals, ParseAmount(t))
			raw = append(raw, t)
		}
		return vals, raw, tokens[:end]
	}

	vals, _, text := figures(true)
	if len(vals) == 0 {
		vals, _, text = figures(false)
	}
	if len(vals) > 3 {
		// More figures than columns: the extra leading ones are part of the narration
		extra := len(vals) - 3
		text = tokens[:len(text)+extra]
		vals = vals[extra:]
	}
	desc = strings.Join(text, " ")

	switch len(vals) {
	case 3: // debit, credit, balance
		debit, credit, balance = math.Abs(vals[0]), math.Abs(vals[1]), vals[2]
	case 2: // amount, balance: the sign or the running balance says which way it went
		balance = vals[1]
		if vals[0] < 0 {
			debit = -vals[0]
		} else {
			credit = vals[0]
		}
	case 1:
		if vals[0] < 0 {
			debit = -vals[0]
		} else {
			credit = vals[0]
		}
	}
	return desc, debit, credit, balance, ref
}

var moneyDecimalRe = regexp.MustCompile(`\.\d{1,2}\)?(?:CR|DR)?$`)

func isDirectionMark(t string) bool {
	return strings.EqualFold(t, "DR") || strings.EqualFold(t, "CR")
}
