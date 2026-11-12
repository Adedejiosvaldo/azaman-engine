package parser

import (
	"log"
	"math"
	"regexp"
	"sort"
	"strings"

	"azaman-engine/internal/models"
	"github.com/ledongthuc/pdf"
)

// Column kinds recognised in a statement table header.
const (
	colTransDate = "trans_date"
	colValueDate = "value_date"
	colReference = "reference"
	colDebit     = "debit"
	colCredit    = "credit"
	colBalance   = "balance"
	colBranch    = "branch"
	colRemarks   = "remarks"
	colAmount    = "amount" // one signed column instead of separate debit and credit
	colParty     = "party"  // "To / From": who the money went to or came from
	colType      = "type"   // says "Debit" / "Credit" for a single amount column
)

// An amount as drawn in a table cell: optional sign and currency, thousands separators or a plain
// decimal, optional CR/DR suffix.
var tableAmountRe = regexp.MustCompile(`^[+-]?\s?(?:[₦N$£€]|NGN|USD|GBP|EUR)?\s?[+-]?\(?(?:\d{1,3}(?:,\d{3})+|\d+)(?:\.\d{1,2})?\)?(?:\s?(?:CR|DR|Cr|Dr))?$`)

type tableColumn struct {
	kind   string
	center float64
}

type tableCell struct {
	x, y, w, size float64
	s             string
}

type tableRow struct {
	y     float64
	cells []tableCell
}

// parseTablePDF reads statements laid out as a table under a header row (GTBank, Zenith, UBA,
// First Bank, Access, Kuda, Moniepoint, PalmPay...). Column positions come from the header, so
// each value lands in the right column no matter how the text was drawn. Wrapped remarks are
// assigned to their transaction by vertical spacing, including lines that run on to the next page.
// Finally every row is reconciled against the running balance, and the totals printed on the
// statement (when it has them) are returned for comparison.
func parseTablePDF(pages [][]pdf.Text, bank, currency string) ([]models.RawTransaction, *models.StatementCheck) {
	var columns []tableColumn
	var results []models.RawTransaction
	var stated *models.StatementCheck
	openingBalance, haveOpening := 0.0, false

	for _, runs := range pages {
		rows := buildTableRows(runs)
		start := 0
		if idx, cols := findTableHeader(rows); idx >= 0 {
			if stated == nil {
				stated = statedTotals(rows[:idx])
			}
			if !haveOpening {
				openingBalance, haveOpening = labelledAmount(rows[:idx], "opening balance", "balance b/f", "balance brought forward")
			}
			columns, start = cols, idx+1
		}
		if len(columns) == 0 {
			if stated == nil {
				stated = statedTotals(rows) // summary on a cover page
			}
			continue // no header seen yet
		}
		body := rows[start:]

		// Rows whose trans-date column holds a date start a transaction
		var dateRows []int
		for i, r := range body {
			if _, _, ok := leadingDate(cellText(r, columns, colTransDate)); ok {
				dateRows = append(dateRows, i)
			} else if !haveOpening && len(dateRows) == 0 && strings.Contains(strings.ToLower(rowText(r)), "opening balance") {
				if v, ok := firstAmount(r, columns, colBalance); ok {
					openingBalance, haveOpening = v, true
				}
			}
		}
		if len(dateRows) == 0 {
			continue
		}

		// Most banks (GTBank included) top-align a row: the date sits on the first remarks line and
		// long remarks wrap downward. Some centre the date instead. Detect which from the data.
		topAligned := 0
		for _, ri := range dateRows {
			if cellText(body[ri], columns, colRemarks) != "" {
				topAligned++
			}
		}
		useTop := topAligned*2 >= len(dateRows)

		blocks, lead := remarkBlocks(body, columns, dateRows)
		if blocks == nil && useTop {
			lead = leadingRemarks(body[:dateRows[0]], columns)
		}
		// Remarks that ran over from the last transaction on the previous page
		if len(lead) > 0 && len(results) > 0 {
			last := &results[len(results)-1]
			last.Description = strings.Join(strings.Fields(last.Description+" "+strings.Join(lead, " ")), " ")
			last.RawLine = last.Date + " | " + last.Description
		}

		for k, ri := range dateRows {
			y := body[ri].y
			var upper, lower float64
			tol := body[ri].lineHeight() / 2
			switch {
			case useTop:
				upper = y + tol
				if k+1 < len(dateRows) {
					lower = body[dateRows[k+1]].y + tol
				} else {
					lower = lastRowBottom(body[ri:], columns) // last row on page: its wrapped lines only
				}
			case len(dateRows) == 1:
				upper, lower = y+body[ri].lineHeight()*3, y-body[ri].lineHeight()*3
			case k == 0:
				half := (y - body[dateRows[k+1]].y) / 2
				upper, lower = y+half, y-half
			case k == len(dateRows)-1:
				half := (body[dateRows[k-1]].y - y) / 2
				upper, lower = y+half, y-half
			default:
				upper = (y + body[dateRows[k-1]].y) / 2
				lower = (y + body[dateRows[k+1]].y) / 2
			}

			tx := models.RawTransaction{Bank: bank, Currency: currency}
			var dateRest string
			tx.Date, dateRest, _ = leadingDate(cellText(body[ri], columns, colTransDate))
			tx.ParsedDate, _ = ParseDate(tx.Date)

			var remarks, refs, party []string
			amount, haveAmount, kind := 0.0, false, ""
			for _, r := range body {
				if r.y > upper || r.y <= lower {
					continue
				}
				for _, c := range r.cells {
					switch nearestColumn(c, columns) {
					case colRemarks:
						remarks = append(remarks, c.s)
					case colParty:
						party = append(party, c.s)
					case colReference:
						refs = append(refs, strings.TrimLeft(c.s, "'"))
					case colDebit:
						if v, ok := parseTableAmount(c.s); ok && tx.Debit == 0 {
							tx.Debit = math.Abs(v)
						}
					case colCredit:
						if v, ok := parseTableAmount(c.s); ok && tx.Credit == 0 {
							tx.Credit = math.Abs(v)
						}
					case colAmount:
						if v, ok := parseTableAmount(c.s); ok && !haveAmount {
							amount, haveAmount = v, true
							if strings.HasPrefix(strings.TrimSpace(c.s), "+") {
								kind = "credit"
							}
						}
					case colType:
						if kind == "" {
							kind = strings.ToLower(c.s)
						}
					case colBalance:
						if v, ok := parseTableAmount(c.s); ok && tx.Balance == 0 {
							tx.Balance = v
						}
					}
				}
			}
			if haveAmount && tx.Debit == 0 && tx.Credit == 0 {
				tx.Debit, tx.Credit = splitSignedAmount(amount, kind)
			}
			if blocks != nil {
				remarks = blocks[k]
			}
			if dateRest != "" {
				remarks = append([]string{dateRest}, remarks...) // text drawn right after the date
			}
			tx.Description = strings.Join(strings.Fields(strings.Join(append(party, remarks...), " ")), " ")
			tx.Reference = strings.Join(refs, " ")
			if tx.Description == "" {
				tx.Description = tx.Reference
			}
			tx.RawLine = tx.Date + " | " + tx.Description
			results = append(results, tx)
		}
	}

	reconcileBalances(results, openingBalance, haveOpening)
	return results, stated
}

// splitSignedAmount turns a single amount column into debit or credit. A type column ("Debit",
// "DR", "Money out") decides first, then the sign; an unsigned amount with no type is taken as a
// credit and corrected from the running balance where there is one.
func splitSignedAmount(amount float64, kind string) (debit, credit float64) {
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(kind, w) {
				return true
			}
		}
		return false
	}
	// Short codes count only as whole words: "subscription" contains "cr", "spending" contains "in".
	word := func(codes ...string) bool {
		for _, f := range strings.FieldsFunc(kind, func(r rune) bool { return r < 'a' || r > 'z' }) {
			for _, c := range codes {
				if f == c {
					return true
				}
			}
		}
		return false
	}
	switch {
	case has("debit", "withdraw", "outflow", "sent", "payment", "purchase", "spend") || word("dr", "out", "d"):
		return math.Abs(amount), 0
	case has("credit", "deposit", "received", "inflow") || word("cr", "in", "c"):
		return 0, math.Abs(amount)
	case amount < 0:
		return -amount, 0
	}
	return 0, amount
}

var leadingDateRes = []*regexp.Regexp{
	// 01-Jan-2024 10:15:22, 01/02/2024 10:15 PM, 2024-01-02T10:15:22
	regexp.MustCompile(`^\d{1,4}[-/.][A-Za-z0-9]{1,3}[-/.]\d{2,4}(?:[ T,]+\d{1,2}:\d{2}(?::\d{2})?(?:\s?[APap][Mm])?)?`),
	// 01 Jan 2024 10:15, 1st January 2024, 01 Jan, 2024 10:15
	regexp.MustCompile(`^\d{1,2}(?:st|nd|rd|th)?\s+[A-Za-z]{3,9},?\s+\d{2,4}(?:[ ,]+\d{1,2}:\d{2}(?::\d{2})?(?:\s?[APap][Mm])?)?`),
	// Jan 2, 2024 10:15 PM
	regexp.MustCompile(`^[A-Za-z]{3,9}\s+\d{1,2},?\s+\d{4}(?:[ ,]+\d{1,2}:\d{2}(?::\d{2})?(?:\s?[APap][Mm])?)?`),
}

// leadingDate finds a date at the start of a cell. The cell is usually just the date, but two
// columns drawn close together arrive as one cell ("01-Jan-2024 01-Jan-2024", or a date followed
// by the first words of the narration). Returns the date text and whatever followed it.
func leadingDate(cell string) (date, rest string, ok bool) {
	cell = strings.TrimSpace(cell)
	if cell == "" {
		return "", "", false
	}
	if _, good := ParseDate(cell); good {
		return cell, "", true
	}
	for _, re := range leadingDateRes {
		m := re.FindString(cell)
		if m == "" {
			continue
		}
		if _, good := ParseDate(m); good {
			rest = strings.TrimSpace(cell[len(m):])
			// A value date drawn next to the trans date is not narration.
			if _, isDate := ParseDate(rest); isDate {
				rest = ""
			}
			return m, rest, true
		}
	}
	return "", "", false
}

// lastRowBottom returns the y below which nothing belongs to the last transaction on a page: its
// wrapped remark lines are evenly spaced, so the first wider gap (page footer, totals) ends it.
func lastRowBottom(rows []tableRow, cols []tableColumn) float64 {
	spacing := rows[0].lineHeight() * 1.7
	bottom := rows[0].y
	for _, r := range rows[1:] {
		if bottom-r.y > spacing {
			break
		}
		for _, c := range r.cells {
			if k := nearestColumn(c, cols); k != colRemarks && k != colReference && k != colParty {
				return bottom - 0.01 // a row of figures or labels: totals, closing balance
			}
		}
		bottom = r.y
	}
	return bottom - 0.01
}

// leadingRemarks returns remark lines above the first transaction of a page: the tail of the
// previous page's last transaction. Rows carrying figures or labels (balance brought forward,
// repeated headers) are not remarks.
func leadingRemarks(rows []tableRow, cols []tableColumn) []string {
	var out []string
	for _, r := range rows {
		lower := strings.ToLower(rowText(r))
		if strings.Contains(lower, "balance") || strings.Contains(lower, "page ") || strings.Contains(lower, "statement") {
			continue
		}
		var parts []string
		plain := true
		for _, c := range r.cells {
			if nearestColumn(c, cols) == colRemarks {
				parts = append(parts, c.s)
			} else {
				plain = false
			}
		}
		if plain && len(parts) > 0 {
			out = append(out, strings.Join(parts, " "))
		}
	}
	return out
}

// statedTotals reads the totals a statement prints about itself ("Total Debit 1,234.00"), from
// the rows above the table. A value sits beside its label or directly beneath it.
func statedTotals(rows []tableRow) *models.StatementCheck {
	debit, ok1 := labelledAmount(rows, "total debit", "total withdrawal", "total money out", "total dr")
	credit, ok2 := labelledAmount(rows, "total credit", "total deposit", "total lodgement", "total money in", "total cr")
	if !ok1 || !ok2 {
		return nil
	}
	return &models.StatementCheck{StatedDebit: math.Abs(debit), StatedCredit: math.Abs(credit)}
}

// labelledAmount finds the amount printed for a label: inside the same cell, the next cell to
// the right, or else beneath it on the next row.
func labelledAmount(rows []tableRow, labels ...string) (float64, bool) {
	for i, r := range rows {
		for j, c := range r.cells {
			lower := strings.ToLower(c.s)
			at := -1
			for _, l := range labels {
				if k := strings.Index(lower, l); k >= 0 {
					at = k + len(l)
					break
				}
			}
			if at < 0 {
				continue
			}
			// "Total Debit: 1,234.00" drawn as one cell
			if tail := strings.Fields(strings.TrimLeft(c.s[at:], "s: ()")); len(tail) > 0 {
				if v, ok := parseTableAmount(tail[len(tail)-1]); ok {
					return v, true
				}
			}
			if j+1 < len(r.cells) {
				if v, ok := parseTableAmount(r.cells[j+1].s); ok {
					return v, true
				}
			}
			if i+1 < len(rows) {
				best, found, dist := 0.0, false, math.Inf(1)
				for _, under := range rows[i+1].cells {
					if v, ok := parseTableAmount(under.s); ok {
						if d := math.Abs(under.x - c.x); d < dist && d < 60 {
							best, found, dist = v, true, d
						}
					}
				}
				if found {
					return best, true
				}
			}
		}
	}
	return 0, false
}

// reconcileBalances uses the running balance to fill in or correct debit/credit: the change in
// balance is the most reliable amount on a statement, since it survives column misplacement.
// Statements listed newest-first are walked from the bottom up.
func reconcileBalances(txs []models.RawTransaction, opening float64, haveOpening bool) {
	if newestFirst(txs) {
		reverse(txs)
		defer reverse(txs)
		haveOpening = false // an opening balance belongs to the oldest row, which is now unknown
	}
	prev, havePrev := opening, haveOpening
	for i := range txs {
		tx := &txs[i]
		if tx.Balance == 0 && tx.Debit == 0 && tx.Credit == 0 {
			continue
		}
		if havePrev && tx.Balance != 0 {
			delta := math.Round((tx.Balance-prev)*100) / 100
			amount := math.Max(tx.Debit, tx.Credit)
			missing := tx.Debit == 0 && tx.Credit == 0
			swapped := amount > 0 && math.Abs(math.Abs(delta)-amount) < 0.01 &&
				((delta < 0 && tx.Debit == 0) || (delta > 0 && tx.Credit == 0))
			if missing || swapped {
				tx.Debit, tx.Credit = 0, 0
				if delta < 0 {
					tx.Debit = -delta
				} else {
					tx.Credit = delta
				}
			}
		}
		if tx.Balance != 0 {
			prev, havePrev = tx.Balance, true
		}
	}
}

func reverse(txs []models.RawTransaction) {
	for i, j := 0, len(txs)-1; i < j; i, j = i+1, j-1 {
		txs[i], txs[j] = txs[j], txs[i]
	}
}

// newestFirst reports whether a statement lists its latest transaction first, judged by which
// direction makes the amounts agree with the running balance, then by the dates.
func newestFirst(txs []models.RawTransaction) bool {
	forward, backward := 0, 0
	for i := 1; i < len(txs); i++ {
		a, b := txs[i-1], txs[i]
		if a.Balance == 0 || b.Balance == 0 {
			continue
		}
		diff := math.Abs(b.Balance - a.Balance)
		if amt := math.Max(b.Debit, b.Credit); amt > 0 && math.Abs(diff-amt) < 0.01 {
			forward++
		}
		if amt := math.Max(a.Debit, a.Credit); amt > 0 && math.Abs(diff-amt) < 0.01 {
			backward++
		}
	}
	if forward != backward {
		return backward > forward
	}
	if n := len(txs); n > 1 && !txs[0].ParsedDate.IsZero() && !txs[n-1].ParsedDate.IsZero() {
		return txs[0].ParsedDate.After(txs[n-1].ParsedDate)
	}
	return false
}

// buildTableRows groups runs into rows by y, and joins runs that sit close together on a row
// into cells (a column value drawn as several Tj calls).
func buildTableRows(runs []pdf.Text) []tableRow {
	sorted := append([]pdf.Text(nil), runs...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Y > sorted[j].Y })

	var rows []tableRow
	for _, g := range sorted {
		if strings.TrimSpace(g.S) == "" {
			continue
		}
		if n := len(rows); n > 0 && math.Abs(g.Y-rows[n-1].y) <= rowTolerance(g) {
			rows[n-1].cells = append(rows[n-1].cells, tableCell{g.X, g.Y, g.W, g.FontSize, g.S})
			continue
		}
		rows = append(rows, tableRow{y: g.Y, cells: []tableCell{{g.X, g.Y, g.W, g.FontSize, g.S}}})
	}

	for i := range rows {
		cells := rows[i].cells
		sort.SliceStable(cells, func(a, b int) bool { return cells[a].x < cells[b].x })
		var merged []tableCell
		for _, c := range cells {
			if n := len(merged); n > 0 {
				last := &merged[n-1]
				size := math.Max(c.size, 6)
				if c.x-(last.x+last.w) <= size*0.6 {
					last.s = strings.TrimSpace(last.s) + " " + strings.TrimSpace(c.s)
					last.w = c.x + c.w - last.x
					continue
				}
			}
			merged = append(merged, c)
		}
		rows[i].cells = merged
	}
	return rows
}

// findTableHeader locates the header row and maps its cells to column kinds. A header names a
// date column and the money columns: debit and credit (under any of their names), or one amount.
func findTableHeader(rows []tableRow) (int, []tableColumn) {
	for i, r := range rows {
		if len(rowText(r)) > 300 {
			continue // a paragraph, not a header
		}
		var cols []tableColumn
		for _, c := range r.cells {
			for _, part := range splitHeaderCell(c) {
				if kind := headerKind(part.s); kind != "" {
					cols = append(cols, tableColumn{kind: kind, center: part.x + part.w/2})
				}
			}
		}
		money := hasKinds(cols, colDebit, colCredit) || hasKinds(cols, colAmount)
		if hasKinds(cols, colTransDate) && money && len(cols) >= 3 {
			return i, cols
		}
	}
	return -1, nil
}

// splitHeaderCell separates header labels that were merged into one cell ("Debits Credits Balance").
// A new label starts at each word that begins a column name; the label's x is estimated from its
// character offset within the cell.
func splitHeaderCell(c tableCell) []tableCell {
	words := strings.Fields(c.s)
	if len(words) <= 1 {
		return []tableCell{c}
	}
	charW := c.w / float64(len([]rune(c.s)))
	var out []tableCell
	var cur []string
	curStart, pos := 0, 0
	flush := func(end int) {
		if len(cur) > 0 {
			out = append(out, tableCell{x: c.x + float64(curStart)*charW, y: c.y,
				w: float64(end-curStart) * charW, size: c.size, s: strings.Join(cur, " ")})
			cur = nil
		}
	}
	for _, w := range words {
		idx := pos + strings.Index(c.s[pos:], w)
		if len(cur) > 0 && startsHeaderLabel(w) {
			flush(idx)
		}
		if len(cur) == 0 {
			curStart = idx
		}
		cur = append(cur, w)
		pos = idx + len(w)
	}
	flush(pos)
	return out
}

func startsHeaderLabel(word string) bool {
	w := strings.ToLower(strings.Trim(word, ".:()"))
	for _, p := range []string{"trans", "value", "ref", "debit", "credit", "withdrawal", "deposit",
		"lodgement", "balance", "originating", "remark", "narration", "description", "details", "posted", "posting",
		"money", "paid", "amount", "category", "date", "particular", "channel", "type", "beneficiary"} {
		if strings.HasPrefix(w, p) {
			return true
		}
	}
	return false
}

func headerKind(label string) string {
	l := strings.ToLower(strings.TrimSpace(label))
	l = strings.Join(strings.Fields(strings.NewReplacer("(", " ", ")", " ", "₦", " ", ".", " ", ":", " ").Replace(l)), " ")
	l = strings.TrimSuffix(l, " ngn")
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(l, w) {
				return true
			}
		}
		return false
	}
	switch {
	case l == "" || len(l) > 40:
		return ""
	case has("value"):
		return colValueDate
	case has("date") || l == "time":
		return colTransDate
	case has("to / from", "to/from", "beneficiary", "counterparty", "sender", "recipient"):
		return colParty
	case has("remark", "narration", "description", "detail", "particular"):
		return colRemarks
	case has("ref") || l == "transaction id" || l == "session id":
		return colReference
	case has("debit", "withdrawal", "money out", "paid out") || l == "dr":
		return colDebit
	case has("credit", "deposit", "lodgement", "money in", "paid in") || l == "cr":
		return colCredit
	case has("balance"):
		return colBalance
	case has("amount"):
		return colAmount
	case has("branch", "channel"):
		return colBranch
	case l == "type" || l == "transaction type" || l == "dr/cr" || l == "cr/dr":
		return colType
	case has("trans", "post"):
		return colTransDate
	}
	return ""
}

func nearestColumn(c tableCell, cols []tableColumn) string {
	center := c.x + c.w/2
	best, bestDist := "", math.Inf(1)
	for _, col := range cols {
		// Left edge also counts: long remarks extend far right of the header's center
		d := math.Min(math.Abs(center-col.center), math.Abs(c.x-col.center))
		if d < bestDist {
			best, bestDist = col.kind, d
		}
	}
	return best
}

func cellText(r tableRow, cols []tableColumn, kind string) string {
	for _, c := range r.cells {
		if nearestColumn(c, cols) == kind {
			return strings.TrimSpace(c.s)
		}
	}
	return ""
}

func firstAmount(r tableRow, cols []tableColumn, kind string) (float64, bool) {
	for _, c := range r.cells {
		if nearestColumn(c, cols) == kind {
			if v, ok := parseTableAmount(c.s); ok {
				return v, true
			}
		}
	}
	return 0, false
}

func parseTableAmount(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if !tableAmountRe.MatchString(s) {
		return 0, false
	}
	return ParseAmount(s), true
}

func rowText(r tableRow) string {
	parts := make([]string, len(r.cells))
	for i, c := range r.cells {
		parts[i] = c.s
	}
	return strings.Join(parts, " ")
}

func (r tableRow) lineHeight() float64 {
	if len(r.cells) > 0 && r.cells[0].size > 0 {
		return r.cells[0].size * 1.2
	}
	return 10
}

func hasKinds(cols []tableColumn, kinds ...string) bool {
	for _, k := range kinds {
		found := false
		for _, c := range cols {
			if c.kind == k {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// remarkBlocks groups the remarks column into one block per transaction using vertical spacing:
// lines inside a cell are evenly spaced, while rows are separated by extra cell padding. This
// works whatever line the date is aligned to (GTBank puts it on the second remarks line).
// Returns nil when the spacing doesn't produce exactly one block per date row, so the caller
// falls back to date-row windows.
func remarkBlocks(body []tableRow, cols []tableColumn, dateRows []int) (perRow [][]string, lead []string) {
	if len(dateRows) < 2 {
		return nil, nil
	}
	type line struct {
		y    float64
		text string
	}
	first := body[dateRows[0]].y + body[dateRows[0]].lineHeight()*4
	var lines []line
	for _, r := range body {
		if r.y > first {
			continue // above the first transaction (opening balance, headers)
		}
		var parts []string
		for _, c := range r.cells {
			if nearestColumn(c, cols) == colRemarks {
				parts = append(parts, c.s)
			}
		}
		if len(parts) > 0 {
			lines = append(lines, line{r.y, strings.Join(parts, " ")})
		}
	}
	if len(lines) < len(dateRows) {
		return nil, nil
	}

	gaps := make([]float64, 0, len(lines)-1)
	for i := 1; i < len(lines); i++ {
		gaps = append(gaps, lines[i-1].y-lines[i].y)
	}
	sortedGaps := append([]float64(nil), gaps...)
	sort.Float64s(sortedGaps)
	base := sortedGaps[len(sortedGaps)/5] // within-cell line spacing (lower quintile)

	var blocks [][]string
	var spans [][2]float64 // top, bottom y of each block
	cur := []string{lines[0].text}
	top := lines[0].y
	for i := 1; i < len(lines); i++ {
		if gaps[i-1] > base*1.3 {
			blocks = append(blocks, cur)
			spans = append(spans, [2]float64{top, lines[i-1].y})
			cur, top = nil, lines[i].y
		}
		cur = append(cur, lines[i].text)
	}
	blocks = append(blocks, cur)
	spans = append(spans, [2]float64{top, lines[len(lines)-1].y})

	// One block too many, sitting wholly above the first date: the tail of the previous page's
	// last transaction.
	if len(blocks) == len(dateRows)+1 {
		first := body[dateRows[0]]
		if spans[0][1] > first.y+first.lineHeight()/2 {
			lead, blocks, spans = blocks[0], blocks[1:], spans[1:]
		}
	}
	if len(blocks) != len(dateRows) {
		log.Printf("[PDF-DIAG] remark blocks fallback: blocks=%d dateRows=%d lines=%d baseGap=%.2f", len(blocks), len(dateRows), len(lines), base)
		return nil, nil
	}
	// Sanity check: each date must sit inside (or right next to) its block
	for k, ri := range dateRows {
		y, tol := body[ri].y, body[ri].lineHeight()
		if y > spans[k][0]+tol || y < spans[k][1]-tol {
			log.Printf("[PDF-DIAG] remark blocks fallback: date row %d outside its block", k)
			return nil, nil
		}
	}
	return blocks, lead
}
