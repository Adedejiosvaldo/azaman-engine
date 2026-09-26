package models

// MerchantBurn summarizes spending for a single merchant in leak reports.
type MerchantBurn struct {
	Merchant   string  `json:"merchant"`
	TotalSpent float64 `json:"total_spent"`
	Count      int     `json:"count"`
	Category   string  `json:"category"`
}

// MidnightLeakReport captures late-night discretionary burn (10 PM - 3 AM).
type MidnightLeakReport struct {
	TotalAmount                float64              `json:"total_amount"`
	Count                      int                  `json:"count"`
	PercentageOfDiscretionary  float64              `json:"percentage_of_discretionary"`
	TopMerchants               []MerchantBurn       `json:"top_merchants"`
	Transactions               []DecodedTransaction `json:"transactions"`
}

// FamilySplit shows breakdown of Urgent 2k into family vs general favors.
type FamilySplit struct {
	FamilyAmount       float64 `json:"family_amount"`
	FamilyCount        int     `json:"family_count"`
	ErrandFavorsAmount float64 `json:"errand_favors_amount"`
	ErrandFavorsCount  int     `json:"errand_favors_count"`
}

// Urgent2kReport aggregates micro-transfers (typically ₦1,000 - ₦15,000).
type Urgent2kReport struct {
	TotalAmount         float64              `json:"total_amount"`
	Count               int                  `json:"count"`
	AverageTransferSize float64              `json:"average_transfer_size"`
	FrequencyPerWeek    float64              `json:"frequency_per_week"`
	FamilyVsErrandSplit FamilySplit          `json:"family_vs_errand_split"`
	Transactions        []DecodedTransaction `json:"transactions"`
}

// DuplicatePair represents suspected duplicate debits.
type DuplicatePair struct {
	Merchant        string             `json:"merchant"`
	Amount          float64            `json:"amount"`
	DifferenceMins  int64              `json:"difference_mins"`
	FirstOccurredAt string             `json:"first_occurred_at"`
	Transactions    []DecodedTransaction `json:"transactions"`
}

// DuplicateDebitsReport captures multiple identical charges within short timeframes.
type DuplicateDebitsReport struct {
	TotalPotentialRefund float64         `json:"total_potential_refund"`
	Count                int             `json:"count"`
	Pairs                []DuplicatePair `json:"pairs"`
}

// BankChargesReport details fees, duties, and alerts extracted from statements.
type BankChargesReport struct {
	TotalCharges    float64              `json:"total_charges"`
	Count           int                  `json:"count"`
	SMSAlerts       float64              `json:"sms_alerts"`
	StampDuty       float64              `json:"stamp_duty"`
	TransferFees    float64              `json:"transfer_fees"`
	MaintenanceFees float64              `json:"maintenance_fees"`
	OtherFees       float64              `json:"other_fees"`
	Transactions    []DecodedTransaction `json:"transactions"`
}

// CashLeaks consolidates all silent leaks.
type CashLeaks struct {
	MidnightLeak                MidnightLeakReport    `json:"midnight_leak"`
	Urgent2k                    Urgent2kReport        `json:"urgent_2k"`
	DuplicateDebits             DuplicateDebitsReport `json:"duplicate_debits"`
	BankCharges                 BankChargesReport     `json:"bank_charges"`
	TotalLeakBurn               float64               `json:"total_leak_burn"`
	TotalLeakPercentageOfOutflow float64              `json:"total_leak_percentage_of_outflow"`
}

// AuditSummary contains high-level KPIs and runway.
type AuditSummary struct {
	TotalInflow     float64 `json:"total_inflow"`
	TotalOutflow    float64 `json:"total_outflow"`
	NetRetained     float64 `json:"net_retained"`
	ProfitMarginPct float64 `json:"profit_margin_pct"`
	RunwayMonths    float64 `json:"runway_months"`
	Currency        string  `json:"currency"`
	TransactionCount int    `json:"transaction_count"`
	PeriodStart     string  `json:"period_start"`
	PeriodEnd       string  `json:"period_end"`
	CurrentBalance  float64 `json:"current_balance"`
	AvgDailyBurn    float64 `json:"avg_daily_burn"`
	AvgMonthlyBurn  float64 `json:"avg_monthly_burn"`
}

// CategoryBreakdown provides aggregated volume per spending category.
type CategoryBreakdown struct {
	Category   string  `json:"category"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
	Count      int     `json:"count"`
}

// AuditMeta contains execution metrics confirming ephemeral compliance.
type AuditMeta struct {
	SessionID       string `json:"session_id"`
	BankDetected    string `json:"bank_detected"`
	ExecutionTimeMs int64  `json:"execution_time_ms"`
	ProcessedAt     string `json:"processed_at"`
	MemoryScrubbed  bool   `json:"memory_scrubbed"`
	FileFormat      string `json:"file_format"`
	FileSize        int64  `json:"file_size"`
}

// AuditReport is the master payload returned by /api/v1/shazam/audit.
type AuditReport struct {
	Summary      AuditSummary        `json:"summary"`
	CashLeaks    CashLeaks           `json:"cash_leaks"`
	Categories   []CategoryBreakdown `json:"categories"`
	Transactions []DecodedTransaction `json:"transactions"`
	BankDetected string              `json:"bank_detected"`
	AuditMeta    AuditMeta           `json:"audit_meta"`
}
