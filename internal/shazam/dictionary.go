package shazam

import "strings"

// MerchantRule represents a signature used to detect and categorize merchants.
type MerchantRule struct {
	Patterns        []string // Keywords (case-insensitive)
	Merchant        string   // Clean display name
	Category        string   // Standardized category
	Classification  string   // Tax-Deductible, Discretionary, etc.
	IsLateNight     bool     // Midnight leak candidate
	IsTechVendor    bool     // Cloud, SaaS, Coworking
	IsFee           bool     // Bank fees/levies
	IsIncome        bool     // Salary, retainers, payouts
	Confidence      float64  // Base confidence score
}

// MerchantRegistry holds all registered rules for Nigerian and international merchants.
var MerchantRegistry = []MerchantRule{
	// ==================== LATE NIGHT & FOOD LEAKS ====================
	{
		Patterns:       []string{"chowdeck", "chow deck"},
		Merchant:       "Chowdeck Delivery",
		Category:       "Late Night Food & Lounges",
		Classification: "Personal Discretionary",
		IsLateNight:    true,
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"glovo"},
		Merchant:       "Glovo Fast Delivery",
		Category:       "Late Night Food & Lounges",
		Classification: "Personal Discretionary",
		IsLateNight:    true,
		Confidence:     0.98,
	},
	{
		Patterns:       []string{"foodcourt", "food court"},
		Merchant:       "FoodCourt Delivery",
		Category:       "Late Night Food & Lounges",
		Classification: "Personal Discretionary",
		IsLateNight:    true,
		Confidence:     0.97,
	},
	{
		Patterns:       []string{"dominos", "domino s", "coldstone", "cold stone"},
		Merchant:       "Domino's Pizza / Cold Stone",
		Category:       "Food & Dining",
		Classification: "Personal Discretionary",
		IsLateNight:    true,
		Confidence:     0.98,
	},
	{
		Patterns:       []string{"bukka hut", "bukkahut"},
		Merchant:       "Bukka Hut Restaurant",
		Category:       "Food & Dining",
		Classification: "Personal Discretionary",
		Confidence:     0.96,
	},
	{
		Patterns:       []string{"the place", "theplace"},
		Merchant:       "The Place Restaurant",
		Category:       "Food & Dining",
		Classification: "Personal Discretionary",
		IsLateNight:    true,
		Confidence:     0.97,
	},
	{
		Patterns:       []string{"chicken republic", "piexpress", "pie xpress"},
		Merchant:       "Chicken Republic",
		Category:       "Food & Dining",
		Classification: "Personal Discretionary",
		Confidence:     0.96,
	},
	{
		Patterns:       []string{"mega chicken", "megachicken"},
		Merchant:       "Mega Chicken",
		Category:       "Food & Dining",
		Classification: "Personal Discretionary",
		Confidence:     0.96,
	},
	{
		Patterns:       []string{"kilimanjaro"},
		Merchant:       "Kilimanjaro Restaurant",
		Category:       "Food & Dining",
		Classification: "Personal Discretionary",
		Confidence:     0.95,
	},
	{
		Patterns:       []string{"tastee fried", "tastee", "de tastee"},
		Merchant:       "Tastee Fried Chicken",
		Category:       "Food & Dining",
		Classification: "Personal Discretionary",
		Confidence:     0.97,
	},
	{
		Patterns:       []string{"addide", "justrite", "just rite", "shoprite", "spar supermarket"},
		Merchant:       "Supermarket & Groceries",
		Category:       "Food & Dining",
		Classification: "Essential Living",
		Confidence:     0.96,
	},
	{
		Patterns:       []string{"choplife", "chop life"},
		Merchant:       "Choplife Bar & Grill",
		Category:       "Late Night Food & Lounges",
		Classification: "Personal Discretionary",
		IsLateNight:    true,
		Confidence:     0.95,
	},
	{
		Patterns:       []string{"lekki lounge", "sailors lounge", "wbar", "w bar", "cubana", "quilox", "bay lounge", "moist beach", "hard rock", "hardrock", "club", "lounge"},
		Merchant:       "Lounge & Nightlife",
		Category:       "Late Night Food & Lounges",
		Classification: "Personal Discretionary",
		IsLateNight:    true,
		Confidence:     0.94,
	},

	// ==================== COMMUTE & TRANSIT ====================
	{
		Patterns:       []string{"bolt", "taxify"},
		Merchant:       "Bolt Ride Hailing",
		Category:       "Commute & Transit",
		Classification: "Personal Discretionary",
		IsLateNight:    true,
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"uber"},
		Merchant:       "Uber Nigeria",
		Category:       "Commute & Transit",
		Classification: "Personal Discretionary",
		IsLateNight:    true,
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"indrive", "in drive", "rida"},
		Merchant:       "InDrive Mobility",
		Category:       "Commute & Transit",
		Classification: "Personal Discretionary",
		Confidence:     0.97,
	},
	{
		Patterns:       []string{"lagos airport", "faan", "mmaid", "terminal 2", "airport"},
		Merchant:       "Lagos Airport Transit",
		Category:       "Commute & Transit",
		Classification: "Personal Discretionary",
		Confidence:     0.96,
	},
	{
		Patterns:       []string{"oando", "totalenergies", "total fuel", "nnpc", "mobil", "conoil", "ardova", "forte oil", "fuel", "petrol"},
		Merchant:       "Fuel Station & Petrol",
		Category:       "Commute & Transit",
		Classification: "Essential Living",
		Confidence:     0.93,
	},
	{
		Patterns:       []string{"shuttlers", "shuttlersmetr"},
		Merchant:       "Shuttlers Corporate Bus Transit",
		Category:       "Commute & Transit",
		Classification: "Tax-Deductible Business Expense",
		Confidence:     0.99,
	},

	// ==================== TECH, CLOUD & WORKSPACE (TAX DEDUCTIBLE) ====================
	{
		Patterns:       []string{"aws", "amazon web", "aws emea"},
		Merchant:       "Amazon Web Services (AWS)",
		Category:       "Tech & Cloud Infrastructure",
		Classification: "Tax-Deductible Business Expense",
		IsTechVendor:   true,
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"github"},
		Merchant:       "GitHub Inc.",
		Category:       "Tech & Cloud Infrastructure",
		Classification: "Tax-Deductible Business Expense",
		IsTechVendor:   true,
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"cursor", "anysphere"},
		Merchant:       "Cursor AI Pro",
		Category:       "Tech & Cloud Infrastructure",
		Classification: "Tax-Deductible Business Expense",
		IsTechVendor:   true,
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"openai", "chatgpt"},
		Merchant:       "OpenAI ChatGPT & API",
		Category:       "Tech & Cloud Infrastructure",
		Classification: "Tax-Deductible Business Expense",
		IsTechVendor:   true,
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"anthropic", "claude.ai"},
		Merchant:       "Anthropic Claude API",
		Category:       "Tech & Cloud Infrastructure",
		Classification: "Tax-Deductible Business Expense",
		IsTechVendor:   true,
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"vercel"},
		Merchant:       "Vercel Cloud Platform",
		Category:       "Tech & Cloud Infrastructure",
		Classification: "Tax-Deductible Business Expense",
		IsTechVendor:   true,
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"digitalocean", "digital ocean"},
		Merchant:       "DigitalOcean Cloud",
		Category:       "Tech & Cloud Infrastructure",
		Classification: "Tax-Deductible Business Expense",
		IsTechVendor:   true,
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"google cloud", "google workspace", "gsuite", "google storage"},
		Merchant:       "Google Workspace & Cloud",
		Category:       "Tech & Cloud Infrastructure",
		Classification: "Tax-Deductible Business Expense",
		IsTechVendor:   true,
		Confidence:     0.98,
	},
	{
		Patterns:       []string{"microsoft", "azure", "office 365", "msft"},
		Merchant:       "Microsoft 365 & Azure",
		Category:       "Tech & Cloud Infrastructure",
		Classification: "Tax-Deductible Business Expense",
		IsTechVendor:   true,
		Confidence:     0.98,
	},
	{
		Patterns:       []string{"figma"},
		Merchant:       "Figma Design",
		Category:       "Tech & Cloud Infrastructure",
		Classification: "Tax-Deductible Business Expense",
		IsTechVendor:   true,
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"vi workspace", "vi_lekki_workspace", "lekki workspace", "venia hub", "cchub", "leadspace", "roamwork", "vibranium valley", "coworking", "workspace"},
		Merchant:       "Co-working Space Hub",
		Category:       "Workspace & Co-working",
		Classification: "Tax-Deductible Business Expense",
		IsTechVendor:   true,
		Confidence:     0.96,
	},
	{
		Patterns:       []string{"starlink", "spacex"},
		Merchant:       "Starlink Satellite Internet",
		Category:       "Workspace & Co-working",
		Classification: "Tax-Deductible Business Expense",
		IsTechVendor:   true,
		Confidence:     0.98,
	},

	// ==================== AIRTIME, DATA & UTILITIES ====================
	{
		Patterns:       []string{"mtn", "vtu/mtn", "mtn vtu"},
		Merchant:       "MTN Airtime & Data",
		Category:       "Airtime & Utilities",
		Classification: "Tax-Deductible Business Expense",
		Confidence:     0.97,
	},
	{
		Patterns:       []string{"airtel", "vtu/airtel"},
		Merchant:       "Airtel Nigeria Data",
		Category:       "Airtime & Utilities",
		Classification: "Tax-Deductible Business Expense",
		Confidence:     0.97,
	},
	{
		Patterns:       []string{"glo", "globacom", "vtu/glo"},
		Merchant:       "Glo Mobile",
		Category:       "Airtime & Utilities",
		Classification: "Tax-Deductible Business Expense",
		Confidence:     0.96,
	},
	{
		Patterns:       []string{"9mobile", "etisalat"},
		Merchant:       "9mobile Telecoms",
		Category:       "Airtime & Utilities",
		Classification: "Tax-Deductible Business Expense",
		Confidence:     0.96,
	},
	{
		Patterns:       []string{"ikeja electric", "ikedc", "eko electric", "ekedc", "aedc", "ibedc", "eedc", "electricity", "nepa", "phcn"},
		Merchant:       "Power Utility & Electricity",
		Category:       "Airtime & Utilities",
		Classification: "Essential Living",
		Confidence:     0.95,
	},

	// ==================== FINTECH & SAVINGS ====================
	{
		Patterns:       []string{"piggyvest", "piggy points"},
		Merchant:       "Piggyvest Savings",
		Category:       "FinTech & Investments",
		Classification: "Personal Discretionary",
		Confidence:     0.98,
	},
	{
		Patterns:       []string{"cowrywise"},
		Merchant:       "Cowrywise Wealth",
		Category:       "FinTech & Investments",
		Classification: "Personal Discretionary",
		Confidence:     0.98,
	},
	{
		Patterns:       []string{"bamboo", "risevest", "trove", "chaka"},
		Merchant:       "Investment Portfolio",
		Category:       "FinTech & Investments",
		Classification: "Personal Discretionary",
		Confidence:     0.96,
	},
	{
		Patterns:       []string{"paystack"},
		Merchant:       "Paystack Checkout",
		Category:       "Shopping & Retail",
		Classification: "Personal Discretionary",
		Confidence:     0.92,
	},
	{
		Patterns:       []string{"owealth", "spend & save", "spend and save", "safebox", "auto-save to owealth"},
		Merchant:       "OPay OWealth Savings & Stash",
		Category:       "FinTech & Investments",
		Classification: "Personal Discretionary",
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"sportybet", "sporty", "betano", "bet9ja", "1xbet", "betting"},
		Merchant:       "Sports Betting & Gaming",
		Category:       "Personal Discretionary",
		Classification: "Personal Discretionary",
		Confidence:     0.98,
	},
	{
		Patterns:       []string{"mountain of fire", "mfm", "redeemed christian", "rccg", "winners chapel", "living faith", "harvesters"},
		Merchant:       "Church Tithe & Offering",
		Category:       "Family & Favors",
		Classification: "Personal Discretionary",
		Confidence:     0.97,
	},

	// ==================== BANK FEES & TAXES ====================
	{
		Patterns:       []string{"sms alert", "sms charge", "sms fee", "alert chg"},
		Merchant:       "Bank SMS Notification Fee",
		Category:       "Bank Fees & Charges",
		Classification: "Bank Charge",
		IsFee:          true,
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"ussd charge", "outstanding ussd charge"},
		Merchant:       "Telecom USSD Banking Levy",
		Category:       "Bank Fees & Charges",
		Classification: "Bank Charge",
		IsFee:          true,
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"emtl", "stamp duty", "electronic money transfer levy"},
		Merchant:       "Federal Stamp Duty (EMTL)",
		Category:       "Bank Fees & Charges",
		Classification: "Bank Charge",
		IsFee:          true,
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"card maintenance", "card mthly", "card fee"},
		Merchant:       "Monthly Debit Card Maintenance",
		Category:       "Bank Fees & Charges",
		Classification: "Bank Charge",
		IsFee:          true,
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"nip chg", "transfer charge", "trf fee", "vat on nip", "vat"},
		Merchant:       "Bank Transfer Surcharge & VAT",
		Category:       "Bank Fees & Charges",
		Classification: "Bank Charge",
		IsFee:          true,
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"token fee", "hardware token", "maintenance fee", "cot", "commission on turnover"},
		Merchant:       "Account Maintenance Fee",
		Category:       "Bank Fees & Charges",
		Classification: "Bank Charge",
		IsFee:          true,
		Confidence:     0.98,
	},

	// ==================== INCOME & RETAINERS ====================
	{
		Patterns:       []string{"geegpay", "geegpay africa"},
		Merchant:       "Geegpay USD Settlement",
		Category:       "Income & Retainers",
		Classification: "Income / Retainer",
		IsIncome:       true,
		Confidence:     0.98,
	},
	{
		Patterns:       []string{"deel"},
		Merchant:       "Deel Contractor Payroll",
		Category:       "Income & Retainers",
		Classification: "Income / Retainer",
		IsIncome:       true,
		Confidence:     0.98,
	},
	{
		Patterns:       []string{"remote usd", "usd conv", "inward remittance", "wire transfer", "payroll", "salary", "retainer"},
		Merchant:       "Client Retainer / Salary",
		Category:       "Income & Retainers",
		Classification: "Income / Retainer",
		IsIncome:       true,
		Confidence:     0.95,
	},

	// ==================== FAMILY & URGENT 2K FAVORS ====================
	{
		Patterns:       []string{"urgent 2k", "urgent2k", "urgent 5k", "urgent 10k"},
		Merchant:       "Urgent 2k Favors",
		Category:       "Family & Favors",
		Classification: "Micro-Transfer (Urgent 2k)",
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"mum", "dad", "mama", "papa", "brother", "sister", "family", "house upkeep", "feeding"},
		Merchant:       "Family Upkeep",
		Category:       "Family & Favors",
		Classification: "Micro-Transfer (Urgent 2k)",
		Confidence:     0.95,
	},
}

// FindRuleByKeyword returns the best matching rule for the cleaned memo.
func FindRuleByKeyword(memo string) *MerchantRule {
	lower := strings.ToLower(memo)
	for i := range MerchantRegistry {
		rule := &MerchantRegistry[i]
		for _, pattern := range rule.Patterns {
			if strings.Contains(lower, pattern) {
				return rule
			}
		}
	}
	return nil
}
