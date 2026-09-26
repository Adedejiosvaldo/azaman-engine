package shazam

import (
	"strings"
	"testing"
	"time"

	"azaman-engine/internal/models"
)

func TestCleanMemo(t *testing.T) {
	tests := []struct {
		input    string
		contains string
	}{
		{
			input:    "TRF 093201/CHOWDECK_LAGOS_NG/UBAS_00",
			contains: "CHOWDECK LAGOS NG",
		},
		{
			input:    "POS W/D 093218 LAGOS AIRPORT TERMINAL 2",
			contains: "LAGOS AIRPORT TERMINAL 2",
		},
		{
			input:    "UBAS_0029 // VI_LEKKI_WORKSPACE",
			contains: "VI LEKKI WORKSPACE",
		},
		{
			input:    "NIP/CR/CLIENT_01_REMOTE_USD_CONV",
			contains: "CLIENT 01 REMOTE USD CONV",
		},
	}

	for _, tt := range tests {
		got := CleanMemo(tt.input)
		if !strings.Contains(got, tt.contains) {
			t.Errorf("CleanMemo(%q) = %q; expected to contain %q", tt.input, got, tt.contains)
		}
	}
}

func TestMaskPII(t *testing.T) {
	input := "NIP/DR/0123456789/JOHN DOE/08031234567"
	masked := MaskPII(input)

	if strings.Contains(masked, "0123456789") {
		t.Errorf("MaskPII failed to mask account number: %s", masked)
	}
	if strings.Contains(masked, "08031234567") {
		t.Errorf("MaskPII failed to mask phone number: %s", masked)
	}
	if !strings.Contains(masked, "****") {
		t.Errorf("MaskPII expected asterisks in output: %s", masked)
	}
}

func TestMatchAndDecode(t *testing.T) {
	testCases := []struct {
		raw            models.RawTransaction
		expectedVendor string
		expectedCat    string
		isMidnight     bool
		isUrgent2k     bool
		isFee          bool
		isIncome       bool
	}{
		{
			raw: models.RawTransaction{
				Description: "TRF 093201/CHOWDECK_LAGOS_NG/UBAS_00",
				Debit:       8500,
			},
			expectedVendor: "Chowdeck Delivery",
			expectedCat:    "Late Night Food & Lounges",
			isMidnight:     true,
		},
		{
			raw: models.RawTransaction{
				Description: "POS W/D 093218 LAGOS AIRPORT TERMINAL 2",
				Debit:       12000,
			},
			expectedVendor: "Lagos Airport Transit",
			expectedCat:    "Commute & Transit",
		},
		{
			raw: models.RawTransaction{
				Description: "UBAS_0029 // VI_LEKKI_WORKSPACE",
				Debit:       45000,
			},
			expectedVendor: "Co-working Space Hub",
			expectedCat:    "Workspace & Co-working",
		},
		{
			raw: models.RawTransaction{
				Description: "NIP/CR/CLIENT_01_REMOTE_USD_CONV",
				Credit:      1500000,
			},
			expectedVendor: "Client Retainer / Salary",
			expectedCat:    "Income & Retainers",
			isIncome:       true,
		},
		{
			raw: models.RawTransaction{
				Description: "NIP/DR/0123456789/ADEBAYO SEGUN/URGENT 2K FOR FOOD",
				Debit:       2000,
				Currency:    "NGN",
			},
			expectedVendor: "Urgent 2k Favors",
			isUrgent2k:     true,
		},
		{
			raw: models.RawTransaction{
				Description: "ELECTRONIC MONEY TRANSFER LEVY (EMTL) / STAMP DUTY",
				Debit:       50,
				Currency:    "NGN",
			},
			expectedVendor: "Federal Stamp Duty (EMTL)",
			isFee:          true,
		},
		{
			raw: models.RawTransaction{
				Description: "BOLT LAGOS TRIP 11PM",
				Debit:       6200,
				ParsedDate:  time.Date(2024, 1, 15, 23, 15, 0, 0, time.UTC),
			},
			expectedVendor: "Bolt Ride Hailing",
			isMidnight:     true,
		},
	}

	for _, tc := range testCases {
		decoded := MatchAndDecode(tc.raw)

		if tc.expectedVendor != "" && !strings.Contains(decoded.Merchant, tc.expectedVendor) {
			t.Errorf("MatchAndDecode(%s) Merchant = %q; expected %q", tc.raw.Description, decoded.Merchant, tc.expectedVendor)
		}
		if tc.expectedCat != "" && decoded.Category != tc.expectedCat {
			t.Errorf("MatchAndDecode(%s) Category = %q; expected %q", tc.raw.Description, decoded.Category, tc.expectedCat)
		}
		if tc.isMidnight && !decoded.Flags.IsMidnightLeak {
			t.Errorf("MatchAndDecode(%s) expected IsMidnightLeak = true", tc.raw.Description)
		}
		if tc.isUrgent2k && !decoded.Flags.IsUrgent2k {
			t.Errorf("MatchAndDecode(%s) expected IsUrgent2k = true", tc.raw.Description)
		}
		if tc.isFee && !decoded.Flags.IsBankFee {
			t.Errorf("MatchAndDecode(%s) expected IsBankFee = true", tc.raw.Description)
		}
		if tc.isIncome && !decoded.Flags.IsIncome {
			t.Errorf("MatchAndDecode(%s) expected IsIncome = true", tc.raw.Description)
		}
	}
}
