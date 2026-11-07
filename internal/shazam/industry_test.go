package shazam

import (
	"testing"

	"azaman-engine/internal/models"
)

func TestIndustryWordsAndItemPurposes(t *testing.T) {
	cases := map[string]string{
		// trade words in business and merchant names
		"Transfer to ROYAL TOUCH UNISEX SALON | Zenith Bank | 9512345678":                       "Personal Care",
		"Transfer to ROSEWOOD PERFUMERY | Kuda MFB | 2012345678":                                "Personal Care",
		"Transfer to SUNRISE FOODS NIGERIA LIMITED - CINEMA LEKKI 1 | MONIE POINT | 6012345678": "Personal Discretionary",
		"Third-Party Merchant Order | BRIGHTCART NIGERIA LIMITED":                               "Shopping & Retail",
		"Transfer to FCMB PUBLIC OFFER | FCMB | 1012345678":                                     "FinTech & Investments",
		// a typed purpose that names the item bought
		"Transfer to GADGETPOINT LTD | Sterling Bank | 8012345678 | Deposits for MacBook M1 max": "Shopping & Retail",
		"Transfer to IBRAHIM GARBA | OPay | 9012345678 | Part Payment. inverter":                 "Shopping & Retail",
		// "TF" is transport fare
		"Transfer to SADE ADE | OPay | 7012345678 | TF": "Commute & Transit",
	}
	for memo, want := range cases {
		got := MatchAndDecode(models.RawTransaction{Description: memo, Debit: 5000})
		if got.Category != want {
			t.Errorf("%s\n  got %s (%.2f), want %s", memo, got.Category, got.Confidence, want)
		}
	}
}
