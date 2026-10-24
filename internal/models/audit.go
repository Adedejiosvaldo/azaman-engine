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
	TotalAmount               float64              `json:"total_amount"`
	Count                     int                  `json:"count"`
	PercentageOfDiscretionary float64              `json:"percentage_of_discretionary"`
	TopMerchants              []MerchantBurn       `json:"top_merchants"`
	Transactions              []DecodedTransaction `json:"transactions"`
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
	Merchant        string               `json:"merchant"`
	Amount          float64              `json:"amount"`
	DifferenceMins  int64                `json:"difference_mins"`
	FirstOccurredAt string               `json:"first_occurred_at"`
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
	EMTL            float64              `json:"emtl"`
	VATOnFees       float64              `json:"vat_on_fees"`
	USSDCharges     float64              `json:"ussd_charges"`
	CyberLevy       float64              `json:"cybersecurity_levy"`
	TotalOvercharge float64              `json:"total_overcharge"`
	Overcharges     []Overcharge         `json:"overcharges"`
	Transactions    []DecodedTransaction `json:"transactions"`
}

// Overcharge is a bank charge that exceeds the published CBN / statutory limit and can be disputed.
type Overcharge struct {
	FeeType string  `json:"fee_type"`
	Date    string  `json:"date"`
	Charged float64 `json:"charged"`
	Allowed float64 `json:"allowed"`
	Excess  float64 `json:"excess"`
	Rule    string  `json:"rule"`
}

// ReversalPair links a failed debit to the credit that reversed it.
type ReversalPair struct {
	GroupID    string  `json:"group_id"`
	Amount     float64 `json:"amount"`
	DebitDate  string  `json:"debit_date"`
	CreditDate string  `json:"credit_date"`
	DelayHours float64 `json:"delay_hours"`
	Late       bool    `json:"late"` // Reversed after the 48h CBN window
}

// ReversalsReport covers failed transfers: reversed ones (excluded from spend) and unresolved ones (dispute candidates).
type ReversalsReport struct {
	ReversedCount    int                  `json:"reversed_count"`
	ReversedAmount   float64              `json:"reversed_amount"`
	LateReversals    int                  `json:"late_reversals"`
	Pairs            []ReversalPair       `json:"pairs"`
	UnresolvedCount  int                  `json:"unresolved_count"`
	UnresolvedAmount float64              `json:"unresolved_amount"`
	UnresolvedDebits []DecodedTransaction `json:"unresolved_debits"`
}

// InternalTransfersReport summarizes money moved between the holder's own accounts and savings wallets.
type InternalTransfersReport struct {
	SelfTransfersOut  float64 `json:"self_transfers_out"`
	SelfTransfersIn   float64 `json:"self_transfers_in"`
	SavingsDeposits   float64 `json:"savings_deposits"`
	SavingsWithdrawal float64 `json:"savings_withdrawals"`
	NetSaved          float64 `json:"net_saved"`
	Count             int     `json:"count"`
}

// RecurringPayment is a same-recipient, similar-amount, regular-interval series.
type RecurringPayment struct {
	Recipient     string  `json:"recipient"`
	Kind          string  `json:"kind"` // "Black Tax (Family Support)", "Ajo / Thrift", "Recurring Payment"
	AverageAmount float64 `json:"average_amount"`
	IntervalDays  float64 `json:"interval_days"`
	Count         int     `json:"count"`
	TotalAmount   float64 `json:"total_amount"`
}

// NigeriaInsights surfaces spending patterns specific to Nigerian money habits.
type NigeriaInsights struct {
	BettingSpend        float64            `json:"betting_spend"`
	BettingCount        int                `json:"betting_count"`
	BettingWinnings     float64            `json:"betting_winnings"`
	AirtimeDataSpend    float64            `json:"airtime_data_spend"`
	LoanDisbursements   float64            `json:"loan_disbursements"`
	LoanRepayments      float64            `json:"loan_repayments"`
	DebtServiceRatioPct float64            `json:"debt_service_ratio_pct"`
	AjoContributions    float64            `json:"ajo_contributions"`
	ReligiousGiving     float64            `json:"religious_giving"`
	BlackTax            float64            `json:"black_tax"`
	SubscriptionsTV     float64            `json:"subscriptions_tv"`
	SavingsRatePct      float64            `json:"savings_rate_pct"`
	RecurringPayments   []RecurringPayment `json:"recurring_payments"`
	Warnings            []string           `json:"warnings"`
}

// TaxEstimate is an indicative personal income tax computation under the Nigeria Tax Act 2025.
type TaxEstimate struct {
	Regime                string  `json:"regime"`
	AnnualizedIncome      float64 `json:"annualized_income"`
	AnnualizedDeductibles float64 `json:"annualized_deductibles"`
	RentRelief            float64 `json:"rent_relief"`
	ChargeableIncome      float64 `json:"chargeable_income"`
	EstimatedAnnualTax    float64 `json:"estimated_annual_tax"`
	EffectiveRatePct      float64 `json:"effective_rate_pct"`
	Note                  string  `json:"note"`
}

// BusinessSplit separates business activity from personal spending (informal traders use personal accounts).
type BusinessSplit struct {
	BusinessInflow  float64 `json:"business_inflow"`
	BusinessOutflow float64 `json:"business_outflow"`
	BusinessNet     float64 `json:"business_net"`
	PersonalInflow  float64 `json:"personal_inflow"`
	PersonalOutflow float64 `json:"personal_outflow"`
}

// CashLeaks consolidates all silent leaks.
type CashLeaks struct {
	MidnightLeak                 MidnightLeakReport    `json:"midnight_leak"`
	Urgent2k                     Urgent2kReport        `json:"urgent_2k"`
	DuplicateDebits              DuplicateDebitsReport `json:"duplicate_debits"`
	BankCharges                  BankChargesReport     `json:"bank_charges"`
	TotalLeakBurn                float64               `json:"total_leak_burn"`
	TotalLeakPercentageOfOutflow float64               `json:"total_leak_percentage_of_outflow"`
}

// AuditSummary contains high-level KPIs and runway.
type AuditSummary struct {
	TotalInflow      float64 `json:"total_inflow"`
	TotalOutflow     float64 `json:"total_outflow"`
	NetRetained      float64 `json:"net_retained"`
	ProfitMarginPct  float64 `json:"profit_margin_pct"`
	RunwayMonths     float64 `json:"runway_months"`
	Currency         string  `json:"currency"`
	TransactionCount int     `json:"transaction_count"`
	PeriodStart      string  `json:"period_start"`
	PeriodEnd        string  `json:"period_end"`
	CurrentBalance   float64 `json:"current_balance"`
	AvgDailyBurn     float64 `json:"avg_daily_burn"`
	AvgMonthlyBurn   float64 `json:"avg_monthly_burn"`
	// WeightedMonthlyBurn weights recent months more heavily so naira inflation is reflected in runway.
	WeightedMonthlyBurn float64 `json:"weighted_monthly_burn"`
	AccountHolder       string  `json:"account_holder,omitempty"`
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
	Summary       AuditSummary            `json:"summary"`
	CashLeaks     CashLeaks               `json:"cash_leaks"`
	Categories    []CategoryBreakdown     `json:"categories"`
	Reversals     ReversalsReport         `json:"reversals"`
	Internal      InternalTransfersReport `json:"internal_transfers"`
	Insights      NigeriaInsights         `json:"nigeria_insights"`
	Tax           TaxEstimate             `json:"tax_estimate"`
	Business      BusinessSplit           `json:"business_split"`
	PeopleReview  []PersonReview          `json:"people_review"`
	Uncategorized UncategorizedReport     `json:"uncategorized"`
	Check         StatementCheck          `json:"statement_check"`
	Grade         FinancialGrade          `json:"grade"`
	SpendingMix   SpendingMix             `json:"spending_mix"`
	Monthly       MonthlyReport           `json:"monthly"`
	Transactions  []DecodedTransaction    `json:"transactions"`
	BankDetected  string                  `json:"bank_detected"`
	AuditMeta     AuditMeta               `json:"audit_meta"`
}

// PersonReview is a counterparty the engine is unsure about (family/friend vs vendor), ranked by
// money sent, with the reasons for its guess. The dashboard asks the user to confirm these.
type PersonReview struct {
	Name      string   `json:"name"`
	Count     int      `json:"count"`
	TotalSent float64  `json:"total_sent"`
	Guess     string   `json:"guess"` // "family", "vendor" or "unsure"
	Score     float64  `json:"score"` // > 0 leans family, < 0 leans vendor
	Reasons   []string `json:"reasons"`
}

// StatementCheck compares what was parsed with the totals printed on the statement, so a misread
// statement is reported instead of silently producing wrong numbers.
type StatementCheck struct {
	Checked      bool    `json:"checked"` // the statement prints totals the parse could be compared with
	OK           bool    `json:"ok"`
	StatedDebit  float64 `json:"stated_debit"`
	StatedCredit float64 `json:"stated_credit"`
	ParsedDebit  float64 `json:"parsed_debit"`
	ParsedCredit float64 `json:"parsed_credit"`
	StatedRows   int     `json:"stated_rows"`
	ParsedRows   int     `json:"parsed_rows"`
	// Method is "totals" (compared with printed totals) or "balance" (each row's amount must
	// carry the running balance from the row before; a skipped or misread row breaks the chain).
	Method        string `json:"method"`
	BalanceBreaks int    `json:"balance_breaks"`
}

// UncategorizedReport totals spending the engine could not categorise with confidence.
type UncategorizedReport struct {
	Count int     `json:"count"`
	Total float64 `json:"total"`
}

// SpendingBucket groups spending by what it does for the account holder.
type SpendingBucket struct {
	Key           string              `json:"key"` // "builders", "essentials", "lifestyle", "drains", "unclassified"
	Label         string              `json:"label"`
	Description   string              `json:"description"`
	Amount        float64             `json:"amount"`
	Percentage    float64             `json:"percentage"`
	Count         int                 `json:"count"`
	TopCategories []CategoryBreakdown `json:"top_categories"`
}

// SpendingMix splits where money went into builders, essentials, lifestyle and drains. Savings
// deposits are internal moves (not spend) but count as builders here, so Total is outflow plus deposits.
type SpendingMix struct {
	Total   float64          `json:"total"`
	Buckets []SpendingBucket `json:"buckets"`
}

// MonthlyFlow is one calendar month of real inflow and outflow (internal moves excluded).
type MonthlyFlow struct {
	Month   string  `json:"month"` // "2006-01"
	Label   string  `json:"label"` // "Jan 2006"
	Inflow  float64 `json:"inflow"`
	Outflow float64 `json:"outflow"`
	Net     float64 `json:"net"`
	Count   int     `json:"count"`
	Partial bool    `json:"partial"` // the statement covers only part of this month
}

// MonthlyReport is the month-by-month cashflow with simple streak stats.
type MonthlyReport struct {
	Months         []MonthlyFlow `json:"months"`
	PositiveMonths int           `json:"positive_months"`
	PositiveStreak int           `json:"positive_streak"` // consecutive net-positive months ending with the latest
	BestMonth      string        `json:"best_month"`
	WorstMonth     string        `json:"worst_month"`
}

// GradeFactor is one scored input to the financial grade.
type GradeFactor struct {
	Key        string  `json:"key"`
	Label      string  `json:"label"`
	Points     float64 `json:"points"`
	MaxPoints  float64 `json:"max_points"`
	Applicable bool    `json:"applicable"` // false when the statement lacks the data (e.g. no income)
	Value      float64 `json:"value"`
	Detail     string  `json:"detail"`
	Tip        string  `json:"tip"`
}

// FinancialGrade is a letter grade (A+ to F) with the factors behind it and the next step up.
type FinancialGrade struct {
	Grade         string        `json:"grade"`
	Score         int           `json:"score"` // 0-100
	Headline      string        `json:"headline"`
	Factors       []GradeFactor `json:"factors"`
	NextGrade     string        `json:"next_grade,omitempty"`
	PointsToNext  float64       `json:"points_to_next,omitempty"`
	Focus         string        `json:"focus,omitempty"` // key of the factor with the most points to gain
	FocusTip      string        `json:"focus_tip,omitempty"`
	LowConfidence bool          `json:"low_confidence"` // statement too short or too sparse to judge
	Note          string        `json:"note,omitempty"`
}
