package parser

import (
	"strings"
	"testing"
	"time"
)

func TestDetectBank(t *testing.T) {
	tests := []struct {
		sample   string
		expected string
	}{
		{"Guaranty Trust Bank Plc - Account Statement", BankGTB},
		{"Kuda Microfinance Bank - Statement of Account", BankKuda},
		{"Access Bank Plc - Transaction Details", BankAccess},
		{"Moniepoint MFB Transaction History", BankMoniepoint},
		{"Geegpay USD Wallet Statement", BankGeegpay},
		{"Standard Statement with Date and Amount", BankGeneric},
	}

	for _, tt := range tests {
		got := DetectBank(tt.sample)
		if got != tt.expected {
			t.Errorf("DetectBank(%q) = %q; expected %q", tt.sample, got, tt.expected)
		}
	}
}

func TestNormalizer(t *testing.T) {
	// Test Date Parsing
	d, ok := ParseDate("15-Jan-2024")
	if !ok || d.Day() != 15 || d.Month() != time.January || d.Year() != 2024 {
		t.Errorf("ParseDate(15-Jan-2024) failed: %v, %v", d, ok)
	}

	d2, ok2 := ParseDate("15/01/2024 23:15:00")
	if !ok2 || d2.Hour() != 23 || d2.Minute() != 15 {
		t.Errorf("ParseDate(15/01/2024 23:15:00) failed: %v, %v", d2, ok2)
	}

	// Test Amount Parsing
	if amt := ParseAmount("₦1,500.00"); amt != 1500.00 {
		t.Errorf("ParseAmount(₦1,500.00) = %f; expected 1500.00", amt)
	}
	if amt := ParseAmount("(2,500.50)"); amt != -2500.50 {
		t.Errorf("ParseAmount((2,500.50)) = %f; expected -2500.50", amt)
	}
	if amt := ParseAmount("5,000.00 DR"); amt != -5000.00 {
		t.Errorf("ParseAmount(5,000.00 DR) = %f; expected -5000.00", amt)
	}
}

func TestParseCSV(t *testing.T) {
	// Sample GTBank CSV format
	gtbCSV := `Trans Date,Value Date,Reference,Debits,Credits,Balance,Remarks
15-Jan-2024,15-Jan-2024,093201,8500.00,,450000.00,TRF 093201/CHOWDECK_LAGOS_NG/UBAS_00
16-Jan-2024,16-Jan-2024,093202,,1500000.00,1950000.00,NIP/CR/CLIENT_01_REMOTE_USD_CONV
17-Jan-2024,17-Jan-2024,093203,2000.00,,1948000.00,NIP/DR/0123456789/ADEBAYO SEGUN/URGENT 2K
`
	txs, err := ParseCSV(strings.NewReader(gtbCSV), BankGTB)
	if err != nil {
		t.Fatalf("ParseCSV failed: %v", err)
	}

	if len(txs) != 3 {
		t.Fatalf("expected 3 transactions, got %d", len(txs))
	}

	if txs[0].Debit != 8500.00 {
		t.Errorf("tx[0] debit = %f, expected 8500.00", txs[0].Debit)
	}
	if txs[1].Credit != 1500000.00 {
		t.Errorf("tx[1] credit = %f, expected 1500000.00", txs[1].Credit)
	}
	if txs[2].Debit != 2000.00 {
		t.Errorf("tx[2] debit = %f, expected 2000.00", txs[2].Debit)
	}
}

func TestParseCSV_SingleAmountColumn(t *testing.T) {
	// Sample Kuda format with single Amount column (negative for debits)
	kudaCSV := `Date,Amount,Description,Category,Balance
2024-01-15,-8500.00,Chowdeck Lagos Late Night,Food,50000.00
2024-01-16,500000.00,Client Freelance Retainer,Income,550000.00
`
	txs, err := ParseCSV(strings.NewReader(kudaCSV), BankKuda)
	if err != nil {
		t.Fatalf("ParseCSV failed: %v", err)
	}

	if len(txs) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(txs))
	}

	if txs[0].Debit != 8500.00 {
		t.Errorf("tx[0] debit = %f, expected 8500.00", txs[0].Debit)
	}
	if txs[1].Credit != 500000.00 {
		t.Errorf("tx[1] credit = %f, expected 500000.00", txs[1].Credit)
	}
}
