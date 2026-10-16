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
)

var tableAmountRe = regexp.MustCompile(`^[₦N]?-?\(?\d{1,3}(?:,\d{3})*(?:\.\d{1,2})?\)?(?:\s?(?:CR|DR))?$`)

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

// parseTablePDF reads statements laid out as a table with a Debit / Credit / Balance header
// (GTBank, Zenith, UBA, First Bank, Access...). Column positions come from the header row, so
// each value lands in the right column no matter how the text was drawn. Wrapped remarks are
// assigned to the nearest transaction by windowing between the midpoints of date rows (GTBank
// centres the date vertically, so remarks wrap both above and below it). Finally every row is
// reconciled against the running balance.
func parseTablePDF(pages [][]pdf.Text, bank, currency string) []models.RawTransaction {
	var columns []tableColumn
	var results []models.RawTransaction
	openingBalance, haveOpening := 0.0, false

	for _, runs := range pages {
		rows := buildTableRows(runs)
		start := 0
		if idx, cols := findTableHeader(rows); idx >= 0 {
			columns, start = cols, idx+1
		}
		if len(columns) == 0 {
			continue // no header seen yet (cover page)
		}
		body := rows[start:]

		// Rows whose trans-date column holds a date start a transaction
		var dateRows []int
		for i, r := range body {
			if _, ok := ParseDate(cellText(r, columns, colTransDate)); ok {
				dateRows = append(dateRows, i)
			} else if !haveOpening && len(dateRows) == 0 && strings.Contains(strings.ToLower(rowText(r)), "opening balance") {
				if v, ok := firstAmount(r, columns, colBalance); ok {
					openingBalance, haveOpening = v, true
				}
			}
		}

		// Most banks (GTBank included) top-align a row: the date sits on the first remarks line and
		// long remarks wrap downward. Some centre the date instead. Detect which from the data.
		topAligned := 0
		for _, ri := range dateRows {
			if cellText(body[ri], columns, colRemarks) != "" {
				topAligned++
			}
		}
		useTop := len(dateRows) > 0 && topAligned*2 >= len(dateRows)

		blocks := remarkBlocks(body, columns, dateRows)

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
					lower = y - body[ri].lineHeight()*6 // last row on page: allow a few wrapped lines
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
			tx.Date = cellText(body[ri], columns, colTransDate)
			tx.ParsedDate, _ = ParseDate(tx.Date)

			var remarks, refs []string
			for _, r := range body {
				if r.y > upper || r.y <= lower {
					continue
				}
				for _, c := range r.cells {
					switch nearestColumn(c, columns) {
					case colRemarks:
						remarks = append(remarks, c.s)
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
					case colBalance:
						if v, ok := parseTableAmount(c.s); ok && tx.Balance == 0 {
							tx.Balance = v
						}
					}
				}
			}
			if blocks != nil {
				remarks = blocks[k]
			}
			tx.Description = strings.Join(strings.Fields(strings.Join(remarks, " ")), " ")
			tx.Reference = strings.Join(refs, " ")
			if tx.Description == "" {
				tx.Description = tx.Reference
			}
			tx.RawLine = tx.Date + " | " + tx.Description
			results = append(results, tx)
		}
	}

	reconcileBalances(results, openingBalance, haveOpening)
	return results
}

// reconcileBalances uses the running balance to fill in or correct debit/credit: the change in
// balance is the most reliable amount on a statement, since it survives column misplacement.
func reconcileBalances(txs []models.RawTransaction, opening float64, haveOpening bool) {
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

// findTableHeader locates the header row and maps its cells to column kinds.
func findTableHeader(rows []tableRow) (int, []tableColumn) {
	for i, r := range rows {
		lower := strings.ToLower(rowText(r))
		hasDebit := strings.Contains(lower, "debit") || strings.Contains(lower, "withdrawal")
		hasCredit := strings.Contains(lower, "credit") || strings.Contains(lower, "deposit") || strings.Contains(lower, "lodgement")
		if !hasDebit || !hasCredit || !strings.Contains(lower, "balance") {
			continue
		}
		var cols []tableColumn
		for _, c := range r.cells {
			for _, part := range splitHeaderCell(c) {
				if kind := headerKind(part.s); kind != "" {
					cols = append(cols, tableColumn{kind: kind, center: part.x + part.w/2})
				}
			}
		}
		if hasKinds(cols, colTransDate, colBalance) {
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
		"lodgement", "balance", "originating", "remark", "narration", "description", "details", "posted", "posting"} {
		if strings.HasPrefix(w, p) {
			return true
		}
	}
	return false
}

func headerKind(label string) string {
	l := strings.ToLower(strings.TrimSpace(label))
	switch {
	case strings.Contains(l, "value"):
		return colValueDate
	case strings.Contains(l, "trans") || strings.Contains(l, "post") || l == "date" || l == "date.":
		return colTransDate
	case strings.Contains(l, "ref"):
		return colReference
	case strings.Contains(l, "debit") || strings.Contains(l, "withdrawal"):
		return colDebit
	case strings.Contains(l, "credit") || strings.Contains(l, "deposit") || strings.Contains(l, "lodgement"):
		return colCredit
	case strings.Contains(l, "balance"):
		return colBalance
	case strings.Contains(l, "branch"):
		return colBranch
	case strings.Contains(l, "remark") || strings.Contains(l, "narration") || strings.Contains(l, "description") || strings.Contains(l, "details"):
		return colRemarks
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
func remarkBlocks(body []tableRow, cols []tableColumn, dateRows []int) [][]string {
	if len(dateRows) < 2 {
		return nil
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
		return nil
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

	if len(blocks) != len(dateRows) {
		log.Printf("[PDF-DIAG] remark blocks fallback: blocks=%d dateRows=%d lines=%d baseGap=%.2f", len(blocks), len(dateRows), len(lines), base)
		return nil
	}
	// Sanity check: each date must sit inside (or right next to) its block
	for k, ri := range dateRows {
		y, tol := body[ri].y, body[ri].lineHeight()
		if y > spans[k][0]+tol || y < spans[k][1]-tol {
			log.Printf("[PDF-DIAG] remark blocks fallback: date row %d outside its block", k)
			return nil
		}
	}
	return blocks
}
