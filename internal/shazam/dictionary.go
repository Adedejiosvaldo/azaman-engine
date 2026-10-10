package shazam

import "strings"

// MerchantRule represents a signature used to detect and categorize merchants.
type MerchantRule struct {
	Patterns       []string // Keywords (case-insensitive)
	Merchant       string   // Clean display name
	Category       string   // Standardized category
	Classification string   // Tax-Deductible, Discretionary, etc.
	IsLateNight    bool     // Midnight leak candidate
	IsTechVendor   bool     // Cloud, SaaS, Coworking
	IsFee          bool     // Bank fees/levies
	IsIncome       bool     // Salary, retainers, payouts
	IsSavings      bool     // Own savings/investment wallet (internal movement, not spend)
	IsLoan         bool     // Loan app (credit = disbursement, debit = repayment)
	IsSelfTransfer bool     // Money moving between the holder's own accounts (not spend or income)
	PurposeOnly    bool     // Generic words ("mama", "rent", "chop"): match the memo purpose, never a person's name
	Confidence     float64  // Base confidence score
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
		Patterns:       []string{"oando", "totalenergies", "total fuel", "nnpc", "mobil oil", "11 plc", "conoil", "ardova", "forte oil", "fuel", "petrol"},
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
		Patterns:       []string{"airtime", "mobile data", "data bundle", "data purchase", "data subscription", "recharge card", "vtu"},
		Merchant:       "Airtime & Data Top-up",
		Category:       "Airtime & Utilities",
		Classification: "Essential Living",
		Confidence:     0.94,
	},
	{
		Patterns: []string{"ikeja electric", "ikedc", "eko electric", "ekedc", "aedc", "ibedc", "eedc", "phed", "kedco", "bedc",
			"jed plc", "electricity", "prepaid meter", "electricity token", "light bill", "nepa", "phcn"},
		Merchant:       "Power Utility & Electricity",
		Category:       "Airtime & Utilities",
		Classification: "Essential Living",
		Confidence:     0.95,
	},

	// ==================== OWN-ACCOUNT MOVEMENTS ====================
	// Bank card used to fund the holder's own fintech wallet ("Add Money" by card). On the bank
	// statement it reads "POSWEB ... OPAY CHARGE LANG ..." and must not be mistaken for a fee.
	{
		Patterns:       []string{"opay charge", "palmpay charge", "kuda charge", "moniepoint charge", "paga charge", "chipper charge", "wallet funding", "fund wallet"},
		Merchant:       "Wallet Top-up (Own Account)",
		Category:       "Internal Transfers",
		Classification: "Self Transfer",
		IsSelfTransfer: true,
		Confidence:     0.97,
	},
	{
		Patterns:       []string{"gt inv. fund", "gt inv fund", "money market fund", "mutual fund", "unit trust", "fixed income fund", "stanbic ibtc asset", "arm investment", "meristem"},
		Merchant:       "Investment Fund",
		Category:       "FinTech & Investments",
		Classification: "Savings & Investment",
		IsSavings:      true,
		Confidence:     0.95,
	},

	// ==================== FINTECH & SAVINGS ====================
	{
		Patterns:       []string{"piggyvest", "piggytech", "piggy points"},
		Merchant:       "Piggyvest Savings",
		Category:       "FinTech & Investments",
		Classification: "Savings & Investment",
		IsSavings:      true,
		Confidence:     0.98,
	},
	{
		Patterns:       []string{"cowrywise"},
		Merchant:       "Cowrywise Wealth",
		Category:       "FinTech & Investments",
		Classification: "Savings & Investment",
		IsSavings:      true,
		Confidence:     0.98,
	},
	{
		Patterns:       []string{"bamboo", "risevest", "rise naira", "trove", "chaka"},
		Merchant:       "Investment Portfolio",
		Category:       "FinTech & Investments",
		Classification: "Savings & Investment",
		IsSavings:      true,
		Confidence:     0.96,
	},
	{
		Patterns:       []string{"palmpay savings", "cashbox", "kuda save", "spend+save", "target savings", "fixed deposit", "moniepoint savings"},
		Merchant:       "Bank Savings Pocket",
		Category:       "FinTech & Investments",
		Classification: "Savings & Investment",
		IsSavings:      true,
		Confidence:     0.95,
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
		Classification: "Savings & Investment",
		IsSavings:      true,
		Confidence:     0.99,
	},
	{
		Patterns: []string{"sportybet", "sporty", "betano", "bet9ja", "1xbet", "betking", "nairabet", "merrybet",
			"bangbet", "msport", "betway", "22bet", "paripesa", "livescore bet", "surebet", "betting"},
		Merchant:       "Sports Betting & Gaming",
		Category:       "Betting & Gaming",
		Classification: "Personal Discretionary",
		IsLateNight:    true,
		Confidence:     0.98,
	},
	{
		// FairMoney, Carbon and Renmoney are also microfinance banks, so their bare names appear as the
		// recipient's bank on ordinary transfers ("Transfer to X | CARBON | 0152..."). Only loan-specific memos match.
		Patterns: []string{"fairmoney loan", "carbon loan", "renmoney loan", "paylater", "palmcredit", "palm credit",
			"branch intl", "branch international", "quickcheck", "okash", "easemoni", "lcredit", "newcredit",
			"loan repayment", "loan disbursement", "loan"},
		Merchant:       "Loan App / Lender",
		Category:       "Loans & Debt Service",
		Classification: "Debt Service",
		IsLoan:         true,
		Confidence:     0.95,
	},
	{
		Patterns:       []string{"ajo", "esusu", "osusu", "adashe", "thrift", "contribution", "cooperative", "coop society"},
		Merchant:       "Ajo / Thrift Contribution",
		Category:       "Ajo & Thrift",
		Classification: "Savings & Investment",
		Confidence:     0.93,
	},
	{
		Patterns: []string{"mountain of fire", "mfm", "redeemed christian", "rccg", "winners chapel", "living faith",
			"harvesters", "deeper life", "christ embassy", "daystar", "ecwa", "church", "tithe", "offering",
			"mosque", "zakat", "sadaqah", "jumat", "nasfat"},
		Merchant:       "Religious Giving (Tithe / Offering / Zakat)",
		Category:       "Religious Giving",
		Classification: "Personal Discretionary",
		Confidence:     0.96,
	},
	{
		Patterns:       []string{"google play", "google playst", "apple.com/bill", "itunes", "dstv", "gotv", "startimes", "multichoice", "showmax", "netflix", "spotify", "apple music", "youtube premium", "boomplay"},
		Merchant:       "TV & Streaming Subscription",
		Category:       "Subscriptions & TV",
		Classification: "Personal Discretionary",
		Confidence:     0.97,
	},
	{
		Patterns:       []string{"school fees", "school fee", "tuition", "waec", "jamb", "neco", "acceptance fee", "pearson vue", "vue testing", "exam fee", "ielts", "toefl", "prometric"},
		Merchant:       "School Fees & Exams",
		Category:       "Education",
		Classification: "Essential Living",
		Confidence:     0.95,
	},
	{
		Patterns:       []string{"house rent", "rent", "caution fee", "agency fee", "lawma", "water bill"},
		Merchant:       "Rent & Housing",
		PurposeOnly:    true,
		Category:       "Rent & Housing",
		Classification: "Essential Living",
		Confidence:     0.92,
	},

	{
		Patterns:       []string{"coralpay", "nextgen pg", "remita", "interswitch", "quickteller", "monnify", "flutterwave", "opay pos", "buypower", "vtpass", "baxi"},
		Merchant:       "Bills & Payment Gateway",
		Category:       "Bills & Online Payments",
		Classification: "Essential Living",
		Confidence:     0.9,
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
		Patterns:       []string{"cybersecurity levy", "cyber security levy", "cyber levy"},
		Merchant:       "Cybersecurity Levy (0.5%)",
		Category:       "Bank Fees & Charges",
		Classification: "Bank Charge",
		IsFee:          true,
		Confidence:     0.99,
	},
	{
		Patterns:       []string{"ussd charge", "outstanding ussd charge", "ussd session", "ussd fee"},
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
		Patterns:       []string{"nip chg", "nip charge", "nip fee", "transfer charge", "transfer fee", "trf fee", "trf chg", "vat on nip", "vat"},
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
		Patterns:       []string{"dividend", "div payment", "interest payment", "coupon payment"},
		Merchant:       "Dividend / Investment Income",
		Category:       "Income & Retainers",
		Classification: "Investment Income",
		IsIncome:       true,
		Confidence:     0.95,
	},
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

	// ==================== PIDGIN / SLANG PURPOSE MEMOS ====================
	{
		Patterns:       []string{"t fare", "tfare", "t-fare", "transport fare", "transpo", "okada", "keke", "danfo", "brt"},
		Merchant:       "Transport Fare",
		PurposeOnly:    true,
		Category:       "Commute & Transit",
		Classification: "Essential Living",
		Confidence:     0.9,
	},
	{
		Patterns:       []string{"chop money", "chop", "food money", "soup money", "market money", "foodstuff", "foodstuffs"},
		Merchant:       "Food & Market Money",
		PurposeOnly:    true,
		Category:       "Food & Dining",
		Classification: "Essential Living",
		Confidence:     0.9,
	},
	{
		Patterns:       []string{"hair", "barbing", "barber", "makeup", "nails", "lashes"},
		Merchant:       "Personal Care & Grooming",
		PurposeOnly:    true,
		Category:       "Personal Care",
		Classification: "Personal Discretionary",
		Confidence:     0.88,
	},

	// ==================== FAMILY & URGENT 2K FAVORS ====================
	{
		Patterns:       []string{"urgent 2k", "urgent2k", "urgent 5k", "urgent 10k", "abeg", "borrow me", "small money", "help me", "sapa", "emergency"},
		Merchant:       "Urgent 2k Favors",
		PurposeOnly:    true,
		Category:       "Family & Favors",
		Classification: "Micro-Transfer (Urgent 2k)",
		Confidence:     0.99,
	},
	{
		Patterns: []string{"mum", "dad", "mama", "papa", "mummy", "daddy", "brother", "sister", "family", "house upkeep", "upkeep",
			"feeding", "iya", "baba", "aunty", "uncle", "pikin", "my wife", "my husband", "black tax",
			"allowance", "gift", "birthday", "sallah", "ileya", "xmas", "christmas", "hospital bill", "support", "assistance", "for you",
			"for the week", "weekly", "happy", "enjoy", "bday", "treatment"},
		Merchant:       "Family Upkeep",
		PurposeOnly:    true,
		Category:       "Family & Favors",
		Classification: "Micro-Transfer (Urgent 2k)",
		Confidence:     0.95,
	},
}

// FindNameRule matches a counterparty's name against merchant rules only (church names, payment
// gateways, known stores), skipping PurposeOnly rules whose generic words would misfire on names.
func FindNameRule(name string) *MerchantRule {
	lower := strings.ToLower(name)
	for i := range MerchantRegistry {
		rule := &MerchantRegistry[i]
		if rule.PurposeOnly {
			continue
		}
		for _, pattern := range rule.Patterns {
			if matchPattern(lower, pattern) {
				return rule
			}
		}
	}
	return nil
}

// FindRuleByKeyword returns the best matching rule for the cleaned memo.
// Short patterns (<= 4 chars, e.g. "glo", "vat", "cot", "mtn") must match a whole word so
// "global", "private" and "scott" do not trigger false positives.
func FindRuleByKeyword(memo string) *MerchantRule {
	lower := strings.ToLower(memo)
	for i := range MerchantRegistry {
		rule := &MerchantRegistry[i]
		for _, pattern := range rule.Patterns {
			if matchPattern(lower, pattern) {
				return rule
			}
		}
	}
	return nil
}

func matchPattern(lower, pattern string) bool {
	if len(pattern) > 4 {
		return strings.Contains(lower, pattern)
	}
	return ContainsWord(lower, pattern)
}

// ContainsWord reports whether lowercase text contains word (or phrase) with word boundaries
// on both sides, so "tax" does not match "taxi" and "mum" does not match "minimum".
func ContainsWord(lower, word string) bool {
	for start := 0; start < len(lower); {
		idx := strings.Index(lower[start:], word)
		if idx < 0 {
			return false
		}
		idx += start
		end := idx + len(word)
		if (idx == 0 || !isWordChar(lower[idx-1])) && (end == len(lower) || !isWordChar(lower[end])) {
			return true
		}
		start = idx + 1
	}
	return false
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}
