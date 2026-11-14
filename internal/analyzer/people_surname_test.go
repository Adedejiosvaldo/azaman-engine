package analyzer

import (
	"testing"

	"azaman-engine/internal/models"
	"azaman-engine/internal/shazam"
)

func TestSharedSurnameNeedsMoreThanOnePerson(t *testing.T) {
	memos := []string{
		"Transfer to KEMI ADUNNI BAKARE | OPay | 1111111111",
		"Transfer to SEUN AYO BAKARE | OPay | 2222222222",
		"Transfer to CHIDI TUNDE | Access Bank | 3333333333",
	}
	var txs []models.DecodedTransaction
	for _, m := range memos {
		txs = append(txs, shazam.MatchAndDecode(models.RawTransaction{Description: m, Debit: 7350}))
	}
	review := ScorePeople(txs, "TUNDE OLUMIDE BAKARE")

	reasonsFor := map[string][]string{}
	for _, r := range review {
		reasonsFor[r.Name] = r.Reasons
	}
	for _, tx := range txs[:2] {
		// Relatives are family support (black tax), an essential: never an Urgent 2k leak.
		if tx.Category != "Black Tax (Family Support)" || tx.Flags.IsUrgent2k {
			t.Errorf("%s: relative sharing the family name should be family support, got %s (urgent %v)",
				tx.Counterparty, tx.Category, tx.Flags.IsUrgent2k)
		}
	}
	for _, r := range reasonsFor["Chidi Tunde"] {
		if r == "shares your family name (Tunde)" {
			t.Errorf("a first name shared with one person is not a family name")
		}
	}
}
