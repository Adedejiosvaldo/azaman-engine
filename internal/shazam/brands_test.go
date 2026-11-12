package shazam

import (
	"testing"

	"azaman-engine/internal/models"
)

func categoryOf(memo string) (string, float64) {
	tx := MatchAndDecode(models.RawTransaction{Description: memo, Debit: 5000, Currency: "NGN"})
	return tx.Category, tx.Confidence
}

func TestBrandsAndTradeWords(t *testing.T) {
	cases := []struct{ memo, want string }{
		// Well-known merchants on card and web rows
		{"POS DEBIT: 20429635207 KFC ADMIRALTY LAGOS NG", "Food & Dining"},
		{"WEB PMT BINANCE 2367369625", "FinTech & Investments"},
		{"FLW*RENMONEY LAGOS NG", "Loans & Debt Service"},
		{"Card Payment | AUDIOMACK", "Subscriptions & TV"},
		{"POS Purchase REDDINGTON HOSPITAL 61309405", "Health & Medical"},
		{"WEB PMT UDEMY 7417341405", "Education"},
		// The gateway says how it was paid; the merchant behind it says what for
		{"PSTK/BUYPOWER.NG/338047486", "Airtime & Utilities"},
		{"WEB PURCHASE: PAYSTACK *CLOUDFLARE", "Tech & Cloud Infrastructure"},
		// The longer name wins
		{"FLW*JUMIA FOOD LAGOS NG", "Late Night Food & Lounges"},
		{"WEB PMT JUMIA 1234509876", "Shopping & Retail"},
		// "mobil" is a filling station, "mobile data" is not
		{"POS Purchase MOBIL FILLING STATION 55120934", "Commute & Transit"},
		{"Mobile Data | 08030000000 | MTN", "Airtime & Utilities"},
		// A trade word beats a generic purpose word ("family", "mama")
		{"POS Purchase FAMILY CARE CLINIC 61309405", "Health & Medical"},
		{"CARD PURCHASE MAMA CASS NG 112835", "Food & Dining"},
		// Businesses paid by transfer, with no "Ltd" or "Ventures" to mark them
		{"Transfer to GOOD SHEPHERD MEDICAL CENTRE", "Health & Medical"},
		{"NIP/DR/5478047790/SMOOTH CUTS BARBERSHOP", "Personal Care"},
		{"Transfer to HOUSE ON THE ROCK | Zenith Bank | 9551513954", "Religious Giving"},
		// Typed purposes
		{"Transfer to FEMI ALABI | OPay | 8031000027 | hospital bill", "Health & Medical"},
		{"Transfer to DAVID ETIM - pocket money", "Family & Favors"},
		{"NIP/DR/3143158263/BEN ONYEKA/refund of money borrowed", "Loans & Debt Service"},
		// Access-style narration: purpose first, then sender and recipient
		{"TRF/lunch/FRM TUNDE BAKARE TO KEMI ADEYEMI", "Food & Dining"},
		{"TRF/Transfer/FRM TUNDE BAKARE TO DE CHOICE FAST FOOD", "Food & Dining"},
		{"NIP/DR/0123456789/ADEBAYO SEGUN/URGENT 2K FOR FOOD", "Family & Favors"}, // the plea outranks the item
	}
	for _, c := range cases {
		if got, conf := categoryOf(c.memo); got != c.want {
			t.Errorf("%q: got %q (%.2f), want %q", c.memo, got, conf, c.want)
		}
	}
}

// A brand name that people also carry decides a card payment, never a transfer to a person.
func TestBrandNamesThatArePeopleNames(t *testing.T) {
	if got, _ := categoryOf("POS DEBIT: 11223344556 ZARA IKEJA CITY MALL LAGOS NG"); got != "Shopping & Retail" {
		t.Errorf("card payment at Zara: got %q", got)
	}
	for _, memo := range []string{
		"Transfer to ZARA AHMED | OPay | 8031000027",
		"NIP/DR/0123456789/NIKE ADEBAYO",
		"Transfer to PATRICIA OKAFOR | Kuda MFB | 2011000045",
	} {
		if got, _ := categoryOf(memo); got == "Shopping & Retail" || got == "FinTech & Investments" {
			t.Errorf("%q is a person, not a shop or an app: got %q", memo, got)
		}
	}
}

func TestSplitMemoFrmTo(t *testing.T) {
	name, purpose := SplitMemo("TRF/house rent/FRM TUNDE BAKARE TO SEUN OKAFOR")
	if name != "SEUN OKAFOR" || purpose != "house rent" {
		t.Errorf("got name=%q purpose=%q", name, purpose)
	}
	if name, purpose = SplitMemo("TRF//FRM TUNDE BAKARE TO KEMI ADEYEMI"); name != "KEMI ADEYEMI" || purpose != "" {
		t.Errorf("empty purpose: got name=%q purpose=%q", name, purpose)
	}
}

// Every brand must be specific enough to stand alone: no duplicates, nothing a line long.
func TestBrandCatalogueIsClean(t *testing.T) {
	seen := map[string]string{}
	for _, g := range brandGroups {
		for _, name := range g.names {
			if len(name) < 3 || len(name) > 40 {
				t.Errorf("brand %q: length out of range", name)
			}
			if other, dup := seen[name]; dup && other != g.category {
				t.Errorf("brand %q is listed under both %q and %q", name, other, g.category)
			}
			seen[name] = g.category
		}
	}
	for name := range cardOnlyBrands {
		if _, ok := seen[name]; !ok {
			t.Errorf("cardOnlyBrands lists %q, which is not in the catalogue", name)
		}
	}
}
