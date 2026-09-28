package parser

import (
	"reflect"
	"testing"

	"github.com/ledongthuc/pdf"
)

func TestGroupRowsRebuildsLinesFromPositions(t *testing.T) {
	// Glyph runs as a Td-positioned statement produces them: same-line cells share y (within
	// tolerance), out of order in the content stream.
	glyphs := []pdf.Text{
		{X: 300, Y: 700.4, FontSize: 8, S: "5,000.00", W: 32},
		{X: 50, Y: 700, FontSize: 8, S: "03-Aug-2026", W: 44},
		{X: 120, Y: 700, FontSize: 8, S: "POS PURCHASE", W: 48},
		{X: 50, Y: 688, FontSize: 8, S: "04-Aug-2026", W: 44},
		{X: 120, Y: 688, FontSize: 8, S: "NIP TRANSFER", W: 48},
	}
	got := groupRows(glyphs)
	want := []string{"03-Aug-2026 POS PURCHASE 5,000.00", "04-Aug-2026 NIP TRANSFER"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMatrixMultiplyAppliesTranslation(t *testing.T) {
	// Text at (10, 20) in a text matrix, page transformed by cm translate(100, 200)
	got := mul([6]float64{1, 0, 0, 1, 10, 20}, [6]float64{1, 0, 0, 1, 100, 200})
	if got[4] != 110 || got[5] != 220 {
		t.Errorf("expected (110, 220), got (%v, %v)", got[4], got[5])
	}
}
