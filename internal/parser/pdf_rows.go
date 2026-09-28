package parser

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/ledongthuc/pdf"
)

// contentRowLines rebuilds text lines for pages where pdf.Page.GetTextByRow collapses.
//
// GetTextByRow only tracks positions set by the Tm operator and ignores Td/TD/T*/cm, which most
// statement generators (GTBank, iText, Jasper) use to move between lines, so every line of a page
// lands at the same y. pdf.Page.Content handles those but panics on fonts without width tables.
// This interpreter tracks the text and graphics matrices itself and never needs glyph widths.
func contentRowLines(page pdf.Page) (lines []string, err error) {
	defer func() {
		if r := recover(); r != nil {
			lines, err = nil, fmt.Errorf("content interpreter: %v", r)
		}
	}()

	contents := page.V.Key("Contents")
	if contents.IsNull() {
		return nil, nil
	}

	encoders := map[string]pdf.TextEncoding{}
	encoderFor := func(name string) pdf.TextEncoding {
		if enc, ok := encoders[name]; ok {
			return enc
		}
		f := page.Font(name)
		enc := f.Encoder()
		encoders[name] = enc
		return enc
	}

	type state struct{ ctm [6]float64 }
	ident := [6]float64{1, 0, 0, 1, 0, 0}
	gs := state{ctm: ident}
	var stack []state
	tm, tlm := ident, ident
	var leading, fontSize float64
	var enc pdf.TextEncoding

	var glyphs []pdf.Text
	show := func(raw string) {
		if enc == nil || raw == "" {
			return
		}
		text := enc.Decode(raw)
		if strings.TrimSpace(text) == "" {
			return
		}
		m := mul(tm, gs.ctm)
		size := fontSize * math.Hypot(m[2], m[3])
		if size <= 0 {
			size = 8
		}
		glyphs = append(glyphs, pdf.Text{X: m[4], Y: m[5], FontSize: size, S: text,
			W: float64(len([]rune(text))) * size * 0.5}) // width estimate; only used for spacing
		// advance roughly so consecutive Tj calls on one line don't overlap
		tm = mul([6]float64{1, 0, 0, 1, float64(len([]rune(text))) * fontSize * 0.5, 0}, tm)
	}
	nextLine := func(tx, ty float64) {
		tlm = mul([6]float64{1, 0, 0, 1, tx, ty}, tlm)
		tm = tlm
	}

	pdf.Interpret(contents, func(stk *pdf.Stack, op string) {
		n := stk.Len()
		args := make([]pdf.Value, n)
		for i := n - 1; i >= 0; i-- {
			args[i] = stk.Pop()
		}
		num := func(i int) float64 { return args[i].Float64() }

		switch op {
		case "q":
			stack = append(stack, gs)
		case "Q":
			if len(stack) > 0 {
				gs = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
		case "cm":
			if n == 6 {
				gs.ctm = mul([6]float64{num(0), num(1), num(2), num(3), num(4), num(5)}, gs.ctm)
			}
		case "BT":
			tm, tlm = ident, ident
		case "Tf":
			if n == 2 {
				enc = encoderFor(args[0].Name())
				fontSize = num(1)
			}
		case "TL":
			if n == 1 {
				leading = num(0)
			}
		case "Td":
			if n == 2 {
				nextLine(num(0), num(1))
			}
		case "TD":
			if n == 2 {
				leading = -num(1)
				nextLine(num(0), num(1))
			}
		case "Tm":
			if n == 6 {
				tlm = [6]float64{num(0), num(1), num(2), num(3), num(4), num(5)}
				tm = tlm
			}
		case "T*":
			nextLine(0, -leading)
		case "Tj":
			if n == 1 {
				show(args[0].RawString())
			}
		case "'":
			nextLine(0, -leading)
			if n == 1 {
				show(args[0].RawString())
			}
		case "\"":
			nextLine(0, -leading)
			if n == 3 {
				show(args[2].RawString())
			}
		case "TJ":
			if n == 1 {
				arr := args[0]
				for i := 0; i < arr.Len(); i++ {
					if v := arr.Index(i); v.Kind() == pdf.String {
						show(v.RawString())
					}
				}
			}
		}
	})

	return groupRows(glyphs), nil
}

// mul multiplies two PDF affine matrices [a b c d e f] (a·b in PDF notation).
func mul(a, b [6]float64) [6]float64 {
	return [6]float64{
		a[0]*b[0] + a[1]*b[2],
		a[0]*b[1] + a[1]*b[3],
		a[2]*b[0] + a[3]*b[2],
		a[2]*b[1] + a[3]*b[3],
		a[4]*b[0] + a[5]*b[2] + b[4],
		a[4]*b[1] + a[5]*b[3] + b[5],
	}
}

// groupRows clusters glyph runs into lines: top-to-bottom by y (within a tolerance), left-to-right by x.
func groupRows(glyphs []pdf.Text) []string {
	sort.SliceStable(glyphs, func(i, j int) bool { return glyphs[i].Y > glyphs[j].Y })

	var lines []string
	var row []pdf.Text
	flush := func() {
		if line := joinRow(row); line != "" {
			lines = append(lines, line)
		}
		row = row[:0]
	}
	for _, g := range glyphs {
		if len(row) > 0 && math.Abs(g.Y-row[0].Y) > rowTolerance(row[0]) {
			flush()
		}
		row = append(row, g)
	}
	flush()
	return lines
}

// joinRow orders a row's glyphs by x and inserts spaces where the horizontal gap is wider than
// a fraction of the font size (word or column boundary).
func joinRow(row []pdf.Text) string {
	if len(row) == 0 {
		return ""
	}
	sorted := append([]pdf.Text(nil), row...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].X < sorted[j].X })

	var b strings.Builder
	prevEnd := math.Inf(-1)
	for _, g := range sorted {
		size := g.FontSize
		if size <= 0 {
			size = 8
		}
		if b.Len() > 0 && g.X-prevEnd > size*0.25 && !strings.HasSuffix(b.String(), " ") && g.S != " " {
			b.WriteByte(' ')
		}
		b.WriteString(g.S)
		prevEnd = g.X + g.W
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func rowTolerance(g pdf.Text) float64 {
	if g.FontSize > 0 {
		return g.FontSize * 0.4
	}
	return 2.5
}

// lineShape masks a line for diagnostics: letters become 'a', digits '9', so layout is visible
// in logs without revealing any statement content.
func lineShape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteByte('9')
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r > 127:
			b.WriteByte('a')
		default:
			b.WriteRune(r)
		}
		if b.Len() >= 90 {
			break
		}
	}
	return b.String()
}
