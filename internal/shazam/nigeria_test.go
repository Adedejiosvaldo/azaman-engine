package shazam

import (
	"testing"
	"time"

	"azaman-engine/internal/models"
)

func TestShortPatternsNeedWordBoundary(t *testing.T) {
	cases := map[string]string{
		"Mobile Data | 07148699843 | Glo":     "Glo Mobile", // "mobil" used to hit Fuel
		"Transfer to GLOBAL LINKS VENTURES":   "",           // "glo" inside "global"
		"Transfer to X | CARBON | 0193411483": "",           // Carbon MFB is a bank, not a loan
		"Betting | 0705 | SPORTYBET":          "Sports Betting & Gaming",
		"Transfer to MFM WEALTH FROM ABOVE":   "Religious Giving (Tithe / Offering / Zakat)",
		"DSTV Premium subscription":           "TV & Streaming Subscription",
		"Palmcredit loan repayment":           "Loan App / Lender",
		"Transfer to Tunde - t fare":          "Transport Fare",
	}
	for memo, want := range cases {
		rule := FindRuleByKeyword(CleanMemo(memo))
		got := ""
		if rule != nil {
			got = rule.Merchant
		}
		if got != want {
			t.Errorf("%q: expected %q, got %q", memo, want, got)
		}
	}
}

func TestDateOnlyStatementIsNotMidnight(t *testing.T) {
	d := MatchAndDecode(models.RawTransaction{
		Description: "Transfer to GBENGA - food",
		Debit:       3000,
		ParsedDate:  time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), // date only, no time column
	})
	if d.Flags.IsMidnightLeak {
		t.Errorf("date-only rows (00:00:00) must not be treated as midnight")
	}
}

func TestLateNightMerchantUsesRealTime(t *testing.T) {
	noon := MatchAndDecode(models.RawTransaction{Description: "Third-Party Merchant Order | Chowdeck", Debit: 4000,
		ParsedDate: time.Date(2026, 6, 11, 12, 24, 0, 0, time.UTC)})
	late := MatchAndDecode(models.RawTransaction{Description: "Third-Party Merchant Order | Chowdeck", Debit: 4000,
		ParsedDate: time.Date(2026, 6, 11, 23, 40, 0, 0, time.UTC)})
	if noon.Flags.IsMidnightLeak {
		t.Errorf("Chowdeck at 12:24 is not a midnight leak")
	}
	if !late.Flags.IsMidnightLeak {
		t.Errorf("Chowdeck at 23:40 is a midnight leak")
	}
}

func TestSavingsAndLoansAreNotIncomeOrFavors(t *testing.T) {
	save := MatchAndDecode(models.RawTransaction{Description: "Auto-save to OWealth Balance", Debit: 5000})
	if !save.Flags.IsSavings || save.Flags.IsUrgent2k {
		t.Errorf("OWealth deposit should be savings, not Urgent 2k: %+v", save.Flags)
	}
	loan := MatchAndDecode(models.RawTransaction{Description: "Palmcredit loan disbursement", Credit: 20000})
	if !loan.Flags.IsLoan || loan.Flags.IsIncome || loan.Category != "Loan Disbursement" {
		t.Errorf("loan disbursement must not be income: %+v %s", loan.Flags, loan.Category)
	}
}

func TestMoniepointRecipientIsVendorNotFamily(t *testing.T) {
	d := MatchAndDecode(models.RawTransaction{
		Description: "Transfer to  BELLO RUKAYAT ASABI | MONIE POINT | 7752139889",
		Debit:       1000,
	})
	if d.Category != VendorCategory || d.Flags.IsUrgent2k {
		t.Errorf("Moniepoint recipient should be a vendor purchase, got %q urgent2k=%v", d.Category, d.Flags.IsUrgent2k)
	}

	fam := MatchAndDecode(models.RawTransaction{
		Description: "Transfer to FAVOUR IFEOLUWA BALOGUN | OPay | 4906137089",
		Debit:       5000,
	})
	if fam.Category != "Family & Favors" || !fam.Flags.IsUrgent2k {
		t.Errorf("OPay personal transfer should stay a favor, got %q", fam.Category)
	}
}

func TestBusinessTransferIsNotUrgent2k(t *testing.T) {
	d := MatchAndDecode(models.RawTransaction{Description: "Transfer to HNG TECH LIMITED | CARBON | 0193411483", Debit: 5000})
	if d.Flags.IsUrgent2k {
		t.Errorf("payment to a company must not be an Urgent 2k favor")
	}
}
