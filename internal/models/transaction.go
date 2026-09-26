package models

import "time"

// TransactionFlags represents boolean tags on a decoded transaction.
type TransactionFlags struct {
	IsMidnightLeak  bool `json:"is_midnight_leak"`
	IsUrgent2k      bool `json:"is_urgent_2k"`
	IsDuplicateDebit bool `json:"is_duplicate_debit"`
	IsBankFee       bool `json:"is_bank_fee"`
	IsIncome        bool `json:"is_income"`
	IsTaxDeductible bool `json:"is_tax_deductible"`
}

// RawTransaction represents an unnormalized transaction extracted directly from a bank statement.
type RawTransaction struct {
	Date        string    `json:"date"`
	ParsedDate  time.Time `json:"parsed_date"`
	Description string    `json:"description"`
	Debit       float64   `json:"debit"`
	Credit      float64   `json:"credit"`
	Balance     float64   `json:"balance"`
	Reference   string    `json:"reference"`
	RawLine     string    `json:"raw_line"`
	Bank        string    `json:"bank"`
	Currency    string    `json:"currency"`
}

// DecodedTransaction represents an enriched, categorized, and scored transaction.
type DecodedTransaction struct {
	ID             string           `json:"id"`
	Date           string           `json:"date"`
	ParsedDate     time.Time        `json:"parsed_date"`
	RawDescription string           `json:"raw_description"` // Masked PII
	CleanMemo      string           `json:"clean_memo"`      // Noise stripped
	Merchant       string           `json:"merchant"`        // Identified vendor / recipient
	Category       string           `json:"category"`        // Category classification
	Classification string           `json:"classification"`  // Expense/Income type
	Confidence     float64          `json:"confidence"`      // Confidence score (0.0 - 1.0)
	TimeWindow     string           `json:"time_window"`     // Inferred time or time string
	Flags          TransactionFlags `json:"flags"`
	Amount         float64          `json:"amount"`          // Negative for debit, positive for credit
	Debit          float64          `json:"debit"`
	Credit         float64          `json:"credit"`
	Balance        float64          `json:"balance"`
	Currency       string           `json:"currency"`
	DuplicateGroup string           `json:"duplicate_group,omitempty"`
}
