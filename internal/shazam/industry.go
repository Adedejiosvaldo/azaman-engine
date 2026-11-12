package shazam

import "strings"

// Trade words in a business or merchant name that say what it sells: "Royal Touch Unisex SALON",
// "Tasty Corner EATERY", "Conoil FILLING STATION". Checked in order, so more specific groups come
// first ("Canteen Shisha LOUNGE" is a lounge, not food). Only words that rarely appear in people's
// names are used, and the answer is the same for every user: no AI, no delay, nothing sent anywhere.
var industryWords = []struct {
	category, classification string
	words                    []string
}{
	// Before food: "Sunrise Foods ... Cinema Lekki" is a cinema ticket.
	{"Personal Discretionary", "Personal Discretionary", []string{"cinema", "cinemas", "filmhouse"}},
	{"Late Night Food & Lounges", "Personal Discretionary", []string{"lounge", "nightclub", "night club", "shisha", "nightlife"}},
	// Before the rest: "Family Care Clinic", "Beauty Dental", "Foodco Pharmacy" are medical.
	{"Health & Medical", "Essential Living", []string{"hospital", "hospitals", "clinic", "clinics", "medical", "medicals", "medicare",
		"pharmacy", "pharmacies", "pharmaceutical", "pharmaceuticals", "chemist", "chemists", "drugstore", "diagnostic", "diagnostics",
		"laboratory", "laboratories", "optical", "opticals", "optics", "optometry", "dental", "dentist", "dentistry", "healthcare",
		"hmo", "maternity", "physiotherapy", "orthopaedic", "paediatric"}},
	{"Personal Care", "Personal Discretionary", []string{"salon", "saloon", "barbing", "barber", "barbers", "barbershop", "unisex", "makeup",
		"cosmetics", "beauty", "nails", "spa", "perfumery", "perfume", "perfumes", "fragrance", "hair", "laundry", "laundromat",
		"dry cleaners", "drycleaners", "gym", "fitness", "skincare"}},
	{"Food & Dining", "Personal Discretionary", []string{"eatery", "restaurant", "restaurants", "buka", "bukka", "kitchen", "bakery", "bakeries",
		"foods", "foodstuff", "fast food", "fastfood", "catering", "suya", "grill", "supermarket", "supermarkets", "cafe", "bistro",
		"pizza", "amala", "shawarma", "confectionery", "confectioneries", "groceries", "grocery", "mart", "provisions"}},
	{"Commute & Transit", "Personal Discretionary", []string{"filling station", "petroleum", "petrol", "energies", "conoil", "motor park", "toll",
		"mechanic", "auto", "autos", "tyres", "tyre", "spare parts", "motors", "car wash", "transport", "airlines", "airways"}},
	{"Education", "Education", []string{"school", "schools", "college", "academy", "tutorial", "tutorials", "university", "polytechnic", "institute"}},
	{"Religious Giving", "Religious Giving", []string{"church", "parish", "mosque", "ministries", "chapel", "cathedral", "tabernacle", "rccg", "zakat"}},
	{"Rent & Housing", "Rent & Housing", []string{"properties", "property", "estates", "realty", "water corporation", "lawma"}},
	{"Betting & Gaming", "Personal Discretionary", []string{"bet", "betting", "gaming", "lotto", "casino"}},
	{"Ajo & Thrift", "Savings", []string{"cooperative", "thrift", "esusu", "multipurpose"}},
	{"FinTech & Investments", "Investment", []string{"public offer", "rights issue"}},
	{"Tech & Cloud Infrastructure", "Operating Expense", []string{"software", "hosting", "technologies", "tech", "cloud"}},
	{"Workspace & Co-working", "Operating Expense", []string{"coworking", "co-working", "workspace"}},
	{"Airtime & Utilities", "Personal Discretionary", []string{"electric", "electricity", "disco", "internet", "gas plant", "broadband"}},
	{"POS Cashout & Agency", "Cash Withdrawal", []string{"atm wdl", "atm withdrawal", "cash withdrawal", "withdrawal", "cashout", "cash out"}},
	{"Bills & Online Payments", "Essential Living", []string{"insurance", "assurance"}},
	{"Personal Discretionary", "Personal Discretionary", []string{"hotel", "hotels", "suites", "resort", "resorts", "apartments", "shortlet"}},
	{"Shopping & Retail", "Personal Discretionary", []string{"fashion", "boutique", "shoes", "shoe", "bags", "bag", "electronics", "gadgets", "gadget", "phones", "phone",
		"tailor", "tailoring", "fabrics", "textiles", "building materials", "jewellery", "jewelry", "wristwatch", "accessories",
		// online shops, and OPay's "Third-Party Merchant Order | <shop>"
		"merchant order", "jumia", "konga", "temu", "aliexpress", "shein",
		// items people type as a transfer purpose ("Deposits for MacBook", "Part payment inverter")
		"laptop", "macbook", "iphone", "inverter", "furniture", "curtain", "curtains", "generator",
		"fridge", "freezer", "television", "mattress", "gown", "clothes", "wig", "sneakers"}},
}

// industryConfidence is "sure" for the rest of the engine (0.9+), so the AI and the
// "Uncategorized" pass leave these rows alone.
const industryConfidence = 0.9

// IndustryCategory returns the category a trade word in name points to, if any.
func IndustryCategory(name string) (category, classification string, ok bool) {
	lower := strings.ToLower(name)
	for _, group := range industryWords {
		for _, w := range group.words {
			if ContainsWord(lower, w) {
				return group.category, group.classification, true
			}
		}
	}
	return "", "", false
}
