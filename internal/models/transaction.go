package models

import "time"

// TransactionFlags represents boolean tags on a decoded transaction.
type TransactionFlags struct {
	IsMidnightLeak   bool `json:"is_midnight_leak"`
	IsUrgent2k       bool `json:"is_urgent_2k"`
	IsDuplicateDebit bool `json:"is_duplicate_debit"`
	IsBankFee        bool `json:"is_bank_fee"`
	IsIncome         bool `json:"is_income"`
	IsTaxDeductible  bool `json:"is_tax_deductible"`
	IsSelfTransfer   bool `json:"is_self_transfer"` // Transfer between the account holder's own accounts
	IsSavings        bool `json:"is_savings"`       // Deposit/withdrawal to a savings or investment wallet (OWealth, PiggyVest...)
	IsReversal       bool `json:"is_reversal"`      // Failed debit and its matching reversal credit
	IsRecurring      bool `json:"is_recurring"`     // Periodic same-amount payment (stipend, ajo, rent, salary)
	IsLoan           bool `json:"is_loan"`          // Loan app disbursement or repayment
	IsBusiness       bool `json:"is_business"`      // Business operating inflow/outflow
	IsOvercharge     bool `json:"is_overcharge"`    // Bank charge above CBN / Nigeria Tax Act limits
}

// IsInternal reports whether the transaction only moves money between the holder's own pockets
// (self-transfers, savings wallets) or is cancelled out by a reversal. Such rows are not real spend or income.
func (f TransactionFlags) IsInternal() bool {
	return f.IsSelfTransfer || f.IsSavings || f.IsReversal
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
	Amount         float64          `json:"amount"` // Negative for debit, positive for credit
	Debit          float64          `json:"debit"`
	Credit         float64          `json:"credit"`
	Balance        float64          `json:"balance"`
	Currency       string           `json:"currency"`
	DuplicateGroup string           `json:"duplicate_group,omitempty"`
	ReversalGroup  string           `json:"reversal_group,omitempty"`
	FeeType        string           `json:"fee_type,omitempty"`
	// Person-to-person transfers: who the other party is, what the memo says the payment was for,
	// and which signal decided the category ("memo", "person", "label", "payment_label").
	Counterparty  string `json:"counterparty,omitempty"`
	Purpose       string `json:"purpose,omitempty"`
	DecisionBasis string `json:"decision_basis,omitempty"`
}

// IsSpend reports whether the transaction is a real outflow (a debit that is not internal).
func (t DecodedTransaction) IsSpend() bool {
	return t.Amount < 0 && t.Debit > 0 && !t.Flags.IsInternal()
}

// IsEarning reports whether the transaction is a real inflow (a credit that is not internal).
func (t DecodedTransaction) IsEarning() bool {
	return t.Credit > 0 && !t.Flags.IsInternal()
}
