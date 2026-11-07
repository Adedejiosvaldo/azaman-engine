package ai

import (
	"fmt"
	"strings"
	"testing"

	"azaman-engine/internal/models"
	"azaman-engine/internal/shazam"
)

// fakeAI records what it was sent and answers every memo with one category.
type fakeAI struct {
	sent     []LayaMemoItem
	category string
	conf     float64
}

func (f *fakeAI) IsAvailable() bool { return true }

func (f *fakeAI) ClassifyBatch(items []LayaMemoItem) (map[string]LayaDecision, error) {
	f.sent = append(f.sent, items...)
	out := map[string]LayaDecision{}
	for _, it := range items {
		out[it.ID] = LayaDecision{ID: it.ID, Category: f.category, Confidence: f.conf}
	}
	return out, nil
}

func decodeAll(memos map[string]float64) []models.DecodedTransaction {
	var txs []models.DecodedTransaction
	for memo, amt := range memos {
		txs = append(txs, shazam.MatchAndDecode(models.RawTransaction{Description: memo, Debit: amt, Currency: "NGN"}))
	}
	return txs
}

func TestCleanForAI(t *testing.T) {
	cases := map[string]string{
		"Transfer to HNG TECH LIMITED | Carbon | 0193411483":         "hng tech limited",
		"POS DEBIT: 20429635207 CHOPLIFE RESTAURANT LAGOS NG":        "choplife restaurant",
		"Transfer to U STAR DIVINE ENT 2 | MONIE POINT | 3752812670": "star divine ent",
		"NIP/DR/0123456789/GOLDEN CRUST BAKERY/bread for TUNDE":      "golden crust bakery bread for",
		"Payment ref 88213411":                                       "",
	}
	for in, want := range cases {
		if got := CleanForAI(in, "Tunde Bakare"); got != want {
			t.Errorf("CleanForAI(%q) = %q, want %q", in, got, want)
		}
	}
	// "First" and "Point" are ordinary words unless they form a bank name.
	if got := CleanForAI("Pepper Point first class kitchen", ""); got != "pepper point first class kitchen" {
		t.Errorf("everyday words removed: %q", got)
	}
}

func TestPassNeverSendsPeopleOrNumbers(t *testing.T) {
	txs := decodeAll(map[string]float64{
		"Transfer to NGOZI AMAKA OBI | OPay | 1560803617":      5000,  // a person: never sent
		"Transfer to OKON GLOBAL CONCEPTS | OPay | 2203948811": 3200,  // a business: sent
		"Transfer to CHIOMA EZE | OPay | 4412093311 | cow leg": 28000, // a person, with purpose: purpose sent
	})
	f := &fakeAI{category: "Food & Dining", conf: 0.895}
	ApplyDecisions(txs, f, "Tunde Bakare")

	var sent []string
	for _, it := range f.sent {
		sent = append(sent, it.Memo)
		if strings.ContainsAny(it.Memo, "0123456789") {
			t.Errorf("numbers reached the AI: %q", it.Memo)
		}
	}
	joined := strings.Join(sent, " | ")
	if strings.Contains(joined, "ngozi") || strings.Contains(joined, "chioma") {
		t.Errorf("a person's name reached the AI: %s", joined)
	}
	if !strings.Contains(joined, "okon global concepts") || !strings.Contains(joined, "cow leg") {
		t.Errorf("expected the business and the purpose to be sent, got: %s", joined)
	}
}

func TestPassSendsBiggestUnknownsFirst(t *testing.T) {
	memos := map[string]float64{}
	for i := 0; i < maxAIItems+10; i++ {
		memos[fmt.Sprintf("Mystery Stall %c%c", 'a'+i%26, 'a'+i/26)] = float64(1000 * (i + 1))
	}
	txs := decodeAll(memos)
	f := &fakeAI{category: "Food & Dining", conf: 0.895}
	stats := ApplyDecisions(txs, f, "")

	if stats.Sent != maxAIItems || stats.Candidates != maxAIItems+10 {
		t.Fatalf("sent %d of %d, want %d of %d", stats.Sent, stats.Candidates, maxAIItems, maxAIItems+10)
	}
	// The smallest amounts are the ones left out.
	for _, it := range f.sent {
		if it.Amount < 11000 {
			t.Errorf("a small memo (%v) was sent while bigger ones waited", it.Amount)
		}
	}
}

func TestPassKeepsRulesWhenAIAbstains(t *testing.T) {
	txs := decodeAll(map[string]float64{"Transfer to OKON GLOBAL CONCEPTS | OPay | 1234567890": 70000})
	before := txs[0].Category
	stats := ApplyDecisions(txs, &fakeAI{category: "Food & Dining", conf: 0}, "")
	if txs[0].Category != before || stats.Changed != 0 {
		t.Errorf("a zero-confidence answer changed %q to %q", before, txs[0].Category)
	}
}
