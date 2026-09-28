package parser

import "testing"

func TestExtractAccountHolder(t *testing.T) {
	cases := map[string]string{
		"Wallet Account Statement,,,,\nAccount Name,CHINEDU OLUMIDE BALOGUN,Account Number,7241845192,,\n": "CHINEDU OLUMIDE BALOGUN",
		"Kuda Microfinance Bank\nAccount Name: TUNDE BAKARE\nCurrency: NGN\n":                              "TUNDE BAKARE",
		"Customer Name: Ada Obi   Account Number: 0123\n":                                                  "Ada Obi",
		"Date,Narration,Debit\n": "",
	}
	for in, want := range cases {
		if got := ExtractAccountHolder(in); got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	}
}

func TestDetectBankPrefersLetterhead(t *testing.T) {
	// A Zenith statement whose narrations mention OPay must still be Zenith
	sample := "ZENITH BANK PLC\nStatement of Account\nDate,Description\n01/01/2026,Transfer to X | OPay | 6646290439\n"
	if got := DetectBank(sample); got != BankZenith {
		t.Errorf("expected %s, got %s", BankZenith, got)
	}
	if got := DetectBank("PalmPay Limited\nTransaction History"); got != BankPalmPay {
		t.Errorf("expected PalmPay, got %s", got)
	}
}

func TestParseAmountNigerianFormats(t *testing.T) {
	cases := map[string]float64{
		"N1,200.00":   1200,
		"NGN 5,000":   5000,
		"1,200.00-":   -1200,
		"-N500":       -500,
		"₦20,224.88":  20224.88,
		"1,500.00 DR": -1500,
		"(250.00)":    -250,
	}
	for in, want := range cases {
		if got := ParseAmount(in); got != want {
			t.Errorf("ParseAmount(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseDateExtraLayouts(t *testing.T) {
	for _, s := range []string{"25-Jan-25 14:03", "25 Jan 2025, 14:03", "25.01.2025", "25 January 2025", "2025-01-25T14:03:00.000Z"} {
		if _, ok := ParseDate(s); !ok {
			t.Errorf("failed to parse %q", s)
		}
	}
}

func TestDetectBankFromFilename(t *testing.T) {
	cases := map[string]string{
		"AC_OKAFOR  CHIOMA ADA_AUGUST 2026_432R005818528_FullStmt.pdf": BankGTB,
		"CHINEDU OLUMIDE BALOGUN_7241845192_20260925221611.pdf":        "",
		"statement.csv": "",
	}
	for name, want := range cases {
		if got := DetectBankFromFilename(name); got != want {
			t.Errorf("%q: want %q, got %q", name, want, got)
		}
	}
}

func TestExtractHolderFromFilename(t *testing.T) {
	cases := map[string]string{
		"AC_BALOGUN  CHINEDU OLUMIDE_AUGUST 2026_432R005818528_FullStmt.pdf": "BALOGUN CHINEDU OLUMIDE",
		"CHINEDU OLUMIDE BALOGUN_7241845192_20260925221611.pdf":              "CHINEDU OLUMIDE BALOGUN",
		"CHINEDU OLUMIDE BALOGUN_7241845192_20260925221611 (1).pdf":          "CHINEDU OLUMIDE BALOGUN",
		"statement.pdf": "",
	}
	for name, want := range cases {
		if got := ExtractHolderFromFilename(name); got != want {
			t.Errorf("%q: want %q, got %q", name, want, got)
		}
	}
}
