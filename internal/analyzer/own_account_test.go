package analyzer

import (
	"testing"

	"azaman-engine/internal/models"
	"azaman-engine/internal/shazam"
)

func TestSameAccountNumberIsOwnAccount(t *testing.T) {
	memos := []string{
		"Transfer from MR TUNDE BAKARE | Zenith Bank | 301****452 | Tf",
		"Transfer from TUNDE | Zenith Bank | 301****452 | Tf",   // name cut short, same account
		"Transfer from TUNDE OKAFOR | GTBank | 555****111 | Tf", // another Tunde, other account
	}
	var txs []models.DecodedTransaction
	for _, m := range memos {
		txs = append(txs, shazam.MatchAndDecode(models.RawTransaction{Description: m, Credit: 20000}))
	}
	DetectSelfTransfers(txs, "TUNDE OLUMIDE BAKARE")
	if !txs[0].Flags.IsSelfTransfer || !txs[1].Flags.IsSelfTransfer {
		t.Errorf("both rows from account 301****452 should be own transfers: %v %v",
			txs[0].Flags.IsSelfTransfer, txs[1].Flags.IsSelfTransfer)
	}
	if txs[2].Flags.IsSelfTransfer {
		t.Errorf("a different Tunde on a different account is not the holder")
	}
}

func TestPhoneNumberInAirtimeIsNotAnAccount(t *testing.T) {
	memos := []string{
		"Transfer from MR TUNDE BAKARE | OPay | 8031****27 | Tf", // own OPay account = phone number
		"Airtime | 8031****27 | Glo",
		"Mobile Data | 8031****27 | Glo | 1GB 3-Days Plan",
	}
	var txs []models.DecodedTransaction
	for i, m := range memos {
		raw := models.RawTransaction{Description: m, Debit: 500}
		if i == 0 {
			raw = models.RawTransaction{Description: m, Credit: 20000}
		}
		txs = append(txs, shazam.MatchAndDecode(raw))
	}
	DetectSelfTransfers(txs, "TUNDE OLUMIDE BAKARE")
	if txs[1].Flags.IsSelfTransfer || txs[2].Flags.IsSelfTransfer {
		t.Errorf("airtime and data for the holder's phone are spending, not own transfers")
	}
}
