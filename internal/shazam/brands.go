package shazam

import "sort"

// Brand catalogue: well-known merchants and billers a Nigerian statement names, grouped by what
// the money bought. Each name becomes a merchant rule (see init). One line here covers a brand
// for every user, with no model and no delay.
//
// Rules for adding a name:
//   - It must identify the business on its own. Words that are also first names, surnames or
//     everyday words need their trade word: "smile communications", not "smile"; "branch
//     international", not "branch".
//   - Names of four letters or fewer only match as whole words ("kfc", "waec"), so they are safe
//     against longer words but not against a person with that name.
//   - Microfinance banks that also lend (FairMoney, Carbon, Renmoney) appear as the recipient's
//     bank on ordinary transfers; list them only with a loan word.
type brandGroup struct {
	category, classification string
	flags                    MerchantRule // only the Is* fields are read
	names                    []string
}

// cardOnlyBrands are brand names people also carry. They decide a card or web payment
// ("POS ZARA IKEJA") but never a transfer to someone called Zara or Nike.
var cardOnlyBrands = map[string]bool{
	"zara": true, "nike": true, "patricia": true, "bata": true, "umba": true, "busha": true, "emirates": true,
	"aviator": true, "rumors": true, "bolivar": true, "subway": true, "tidal": true, "luno": true, "kuts": true,
	"noun": true, "accion": true, "baobab": true, "lidya": true, "aella": true, "specta": true, "docker": true,
	"adobe": true, "canva": true, "framer": true, "linode": true, "postman": true, "mobil": true, "adidas": true,
	"itel": true, "tecno": true, "gadget": true, "landlord": true, "thanksgiving": true, "massage": true,
	"mechanic": true, "renmoney": true, "lapo": true, "hisa": true, "maternity": true,
}

var brandGroups = []brandGroup{
	{category: "Food & Dining", classification: "Personal Discretionary", names: []string{
		"kfc", "burger king", "pizza hut", "debonairs", "tantalizers", "sweet sensation", "mr biggs", "mr bigg", "item 7",
		"item7", "chicken capitol", "krispy kreme", "pinkberry", "sooyah bistro", "cafe one", "art cafe", "mama cass",
		"mamacass", "ofada boy", "jevinik", "yakoyo", "amala shitta", "amala sky", "white house restaurant",
		"ocean basket", "rhapsody", "cactus restaurant", "hans and rene", "hans & rene", "eric kayser", "my coffee",
		"cafe neo", "starbucks", "mcdonald", "subway", "papa johns", "crunchies", "genesis restaurant", "mat ice",
		"chicken and chips", "shawarma", "barcelos", "galitos", "native pot", "southern sun restaurant", "food republic",
	}},
	{category: "Food & Dining", classification: "Essential Living", names: []string{
		"foodco", "hubmart", "market square", "marketsquare", "ebeano", "blenco", "prince ebeano", "grand square",
		"next cash and carry", "sahad stores", "h medix", "bokku", "renee mall", "twinsfaja", "cedi supermarket",
		"jendol", "roban stores", "everyday supermarket", "justrite", "globus supermarket", "supermart.ng", "pricepally",
		"mano.ng", "food concepts", "spar nigeria", "spar",
	}},
	{category: "Late Night Food & Lounges", classification: "Personal Discretionary", flags: MerchantRule{IsLateNight: true}, names: []string{
		"jumia food", "heyfood", "hey food", "bheerhugz", "the house bar", "shisha room", "club 57", "silk club", "zaza lounge",
		"vaniti", "cocoon lounge", "escape nightlife", "rumors", "sip lounge", "the vault lounge", "farm city", "bolivar",
		"bar and grill", "beer parlour", "beer garden", "pepper soup", "isi ewu", "nkwobi",
	}},
	{category: "Commute & Transit", classification: "Essential Living", names: []string{
		"lagride", "gokada", "max.ng", "max okada", "oride", "treepz", "gigm", "god is good motors", "abc transport",
		"peace mass", "guo transport", "chisco", "young shall grow", "libra motors", "ifesinachi", "cross country",
		"cowry card", "cowry topup", "touch and pay", "lcc toll", "lekki concession", "e-tag", "lagos ferry", "lagferry",
		"mrs oil", "rainoil", "enyo retail", "nipco", "bovas", "northwest petroleum", "aa rano", "matrix energy",
		"eterna", "heyden", "ascon oil", "fatgbems", "petrocam", "mobil", "ap filling", "total filling",
		"air peace", "arik air", "ibom air", "dana air", "united nigeria airlines", "green africa", "aero contractors",
		"max air", "overland airways", "valuejet", "wakanow", "travelstart", "travelbeta", "ethiopian airlines",
		"emirates", "qatar airways", "british airways", "turkish airlines", "kenya airways", "rwandair", "asky",
		"car wash", "vulcanizer", "mechanic", "autochek", "carloha", "frsc number plate",
	}},
	{category: "Airtime & Utilities", classification: "Essential Living", names: []string{
		"spectranet", "smile communications", "smile 4g", "fiberone", "fibre one", "ipnx", "tizeti", "wifi.ng", "swift networks",
		"coollink", "ntel", "cobranet", "mainone", "buypower", "vtpass", "ikeja electric", "eko electricity", "abuja electricity",
		"ibadan electricity", "port harcourt electricity", "kano electricity", "enugu electricity", "benin electricity",
		"jos electricity", "kaduna electric", "yola electricity", "aba power", "mtn nigeria", "airtel networks", "glo mobile",
		"cooking gas", "gas refill", "gas plant", "techno gas", "lpg",
	}},
	{category: "Subscriptions & TV", classification: "Personal Discretionary", names: []string{
		"prime video", "amazon prime", "disney plus", "disney+", "hbo max", "apple tv", "apple.com bill", "apple com bill",
		"icloud", "google one", "google youtube", "youtube music", "youtubepremium", "audiomack", "deezer", "tidal",
		"mubi", "crunchyroll", "paramount+", "tstv", "kwese", "irokotv", "iroko tv", "linkedin premium", "x premium",
		"twitter blue", "telegram premium", "snapchat+", "tinder", "bumble", "duolingo plus", "medium.com", "substack",
		"patreon", "onlyfans", "playstation plus", "xbox game pass", "nintendo", "google play",
	}},
	{category: "Tech & Cloud Infrastructure", classification: "Tax-Deductible Business Expense", flags: MerchantRule{IsTechVendor: true}, names: []string{
		"amazon web services", "namecheap", "whogohost", "hostinger", "cloudflare", "godaddy", "bluehost", "siteground",
		"qservers", "smartweb", "garanntor", "truehost", "hetzner", "linode", "vultr", "ovhcloud", "contabo", "render.com",
		"netlify", "heroku", "railway.app", "fly.io", "supabase", "firebase", "mongodb", "planetscale", "notion labs",
		"notion.so", "slack technologies", "zoom.us", "zoom video", "jetbrains", "adobe", "canva", "atlassian", "jira",
		"trello", "linear.app", "postman", "docker", "gitlab", "bitbucket", "sentry.io", "datadog", "twilio", "sendgrid",
		"mailchimp", "termii", "hubspot", "zapier", "airtable", "webflow", "framer", "squarespace", "wix.com", "shopify",
		"wordpress", "midjourney", "perplexity", "elevenlabs", "replit", "huggingface", "grammarly", "dropbox",
		"1password", "lastpass", "nordvpn", "expressvpn", "surfshark", "apple developer", "google ads", "meta ads",
		"facebook ads", "facebk", "upwork", "fiverr", "paystack subscription", "termius", "ngrok", "tailscale",
	}},
	{category: "Betting & Gaming", classification: "Personal Discretionary", flags: MerchantRule{IsLateNight: true}, names: []string{
		"betwinner", "baba ijebu", "premier lotto", "ilotbet", "ilot bet", "football.com", "betbonanza", "bet bonanza",
		"accessbet", "access bet", "betland", "lionsbet", "zebet", "wazobet", "frapapa", "betfarm", "naijabet", "bet365",
		"melbet", "parimatch", "betcorrect", "cloudbet", "stake.com", "hallabet", "winners golden bet", "golden chance lotto",
		"western lotto", "green lotto", "give n take", "afribet", "bet9", "virtual football", "aviator", "pool betting",
	}},
	{category: "Loans & Debt Service", classification: "Debt Service", flags: MerchantRule{IsLoan: true}, names: []string{
		"aella", "specta", "xcredit", "page financials", "zedvance", "credit direct", "kredibank", "kredi bank", "lapo",
		"creditwise", "credit wise", "fairmoney repayment", "carbon repayment", "renmoney repayment", "renmoney",
		"branch loan", "ease moni", "9credit", "kashkash", "sokoloan", "soko loan", "lidya", "payhippo", "moneypal",
		"umba", "irorun", "blocka cash", "quickbucks", "quick bucks", "fint loan", "p2vest", "alat loan", "gtb quick credit",
		"quick credit", "salary advance", "payday loan", "overdraft interest", "loan interest", "auto repayment",
		"creditville", "rosabon", "fast credit", "baobab", "letshego", "accion", "nirsal",
	}},
	{category: "FinTech & Investments", classification: "Savings & Investment", flags: MerchantRule{IsSavings: true}, names: []string{
		"binance", "bybit", "quidax", "luno", "busha", "yellow card", "yellowcard", "kucoin", "okx", "bitget", "roqqu",
		"patricia", "breet", "cowry wise", "investnow", "invest now", "chapel hill denham", "stanbic ibtc asset",
		"united capital", "afrinvest", "cardinalstone", "coronation asset", "fbnquest", "cordros", "vetiva", "meristem",
		"i-invest", "carbon invest", "kuda invest", "risevest", "hisa", "gotrade", "optimus by afrinvest", "wealth.ng",
		"crowdyvest", "farmcrowdy", "thrive agric", "piggy vest", "savings plan", "target savings", "fixed deposit",
		"treasury bill", "treasury bills", "fgn bond", "savings bond", "commercial paper", "kuda save", "pocket top up",
		"flex naira", "flex dollar", "safelock", "pension contribution", "stanbic ibtc pension", "arm pension",
	}},
	{category: "Shopping & Retail", classification: "Personal Discretionary", names: []string{
		"jumia", "konga", "temu", "shein", "aliexpress", "amazon.com", "amazon mktp", "ebay", "asos", "slot systems",
		"slot nigeria", "pointek", "3chub", "3c hub", "pep stores", "pep store", "mr price", "mrprice", "miniso", "zara",
		"nike", "adidas", "h&m", "jiji", "kara.com", "fouani", "istore", "samsung experience", "samsung store",
		"lg brand shop", "balogun market", "ruff n tumble", "casabella", "super savers", "the palms", "ikeja city mall",
		"lifemate", "vitafoam", "mouka", "hisense", "binatone", "skit store", "obeezi", "payporte", "paypal", "wish.com",
		"computer village", "gadget", "tecno", "infinix", "itel", "apple store", "bata", "max fashion", "woolworths",
		"lc waikiki", "swatch", "hugo boss", "polo avenue", "david wej", "kilishi", "leatherworld", "aso ebi", "asoebi",
	}},
	{category: "Health & Medical", classification: "Essential Living", names: []string{
		"medplus", "healthplus", "health plus", "alpha pharmacy", "nett pharmacy", "hmedix", "mopheth", "juli pharmacy",
		"bydow pharmacy", "reddington", "lagoon hospital", "evercare", "st nicholas hospital", "eko hospital", "paelon",
		"first cardiology", "cedarcrest", "nizamiye", "garki hospital", "primus hospital", "duchess hospital",
		"synlab", "clina lancet", "clina-lancet", "pathcare", "mecure", "afriglobal", "medbury", "echolab", "union diagnostic",
		"reliance hmo", "hygeia", "avon hmo", "axa mansard health", "leadway health", "total health trust", "bastion hmo",
		"clearline hmo", "hmo premium", "eye foundation", "smile 360", "dental clinic", "luth", "lasuth", "uch ibadan",
		"teaching hospital", "general hospital", "medical centre", "medical center", "specialist hospital", "maternity",
	}},
	{category: "Personal Care", classification: "Personal Discretionary", names: []string{
		"oriki spa", "zaron", "house of tara", "bm pro", "nuban beauty", "i-fitness", "ifitness", "bodyline", "proflex gym",
		"fitness central", "garment care", "washman", "laundry", "dry clean", "dry cleaner", "dry cleaners", "kuts",
		"barber shop", "barbershop", "barbing salon", "hair palace", "hair studio", "nail studio", "nails studio",
		"makeover", "skincare", "skin care", "massage", "pedicure", "manicure", "gym membership",
	}},
	{category: "Personal Discretionary", classification: "Personal Discretionary", names: []string{
		"genesis cinema", "genesis cinemas", "silverbird", "viva cinema", "ozone cinema", "kada cinema", "imax",
		"playstation network", "playstation store", "steam games", "steampowered", "epic games", "xbox", "landmark beach",
		"omu resort", "funtasticland", "hi-impact", "upbeat centre", "ticketmaster", "tix africa", "tix.africa", "nairabox",
		"afritickets", "radisson", "eko hotel", "eko hotels", "sheraton", "marriott", "transcorp hilton", "protea hotel",
		"oriental hotel", "federal palace", "wheatbaker", "booking.com", "airbnb", "hotels.ng", "expedia", "agoda",
		"la campagne", "inagbe", "tarkwa bay", "lekki conservation", "nike art gallery", "terra kulture",
	}},
	{category: "Bills & Online Payments", classification: "Essential Living", names: []string{
		"firs", "lirs", "lagos state internal revenue", "lasg revenue", "nimc", "immigration passport", "nis passport",
		"frsc", "drivers licence", "driver license", "cac registration", "corporate affairs", "land use charge",
		"tenement rate", "leadway assurance", "axa mansard", "aiico", "custodian insurance", "cornerstone insurance",
		"nem insurance", "mutual benefits", "sovereign trust", "consolidated hallmark", "heirs insurance", "heirs life",
		"general insurance", "insurance premium", "motor insurance", "vehicle licence", "tax payment", "paye remittance",
		"court fees", "notary", "dhl", "fedex", "ups nigeria", "gig logistics", "kwik delivery", "sendbox", "nipost",
	}},
	{category: "Education", classification: "Essential Living", names: []string{
		"ulesson", "coursera", "udemy", "altschool", "alt school", "duolingo", "british council", "niit", "aptech",
		"edx.org", "pluralsight", "udacity", "datacamp", "skillshare", "masterclass", "frontend masters", "codecademy",
		"unilag", "university of lagos", "covenant university", "babcock", "futa", "uniben", "unilorin", "lasu", "oau",
		"noun", "national open university", "yabatech", "greensprings", "corona schools", "chrisland", "grange school",
		"lekki british", "atlantic hall", "whitesands", "loyola jesuit", "nysc", "pta levy", "school uniform", "textbook",
		"textbooks", "lesson fee", "lesson teacher", "extra lesson", "exam registration", "post utme", "sat exam",
		"gre exam", "gmat", "ican", "cibn", "cipm", "nim exam", "project supervision", "hostel fee", "departmental dues",
	}},
	{category: "Religious Giving", classification: "Personal Discretionary", names: []string{
		"house on the rock", "coza", "commonwealth of zion", "elevation church", "daystar christian", "this present house",
		"fountain of life", "trinity house", "guiding light", "salvation ministries", "dunamis", "omega fire",
		"lord's chosen", "lords chosen", "christ apostolic", "celestial church", "cherubim and seraphim", "foursquare",
		"assemblies of god", "anglican communion", "catholic archdiocese", "catholic church", "methodist church",
		"baptist church", "presbyterian", "apostolic faith", "ansar ud deen", "ansarudeen", "nawair ud deen", "muslim ummah",
		"central mosque", "first fruit", "firstfruit", "thanksgiving", "seed offering", "building fund", "welfare offering",
		"sadaqa", "zakah",
	}},
	{category: "Rent & Housing", classification: "Essential Living", names: []string{
		"service charge", "estate dues", "estate levy", "security levy", "landlord", "house rent", "shop rent",
		"waste bill", "refuse bill", "lagos water", "water corporation", "legal and agency", "tenancy",
	}},
	{category: "POS Cashout & Agency", classification: "Cash Withdrawal", names: []string{
		"cardless withdrawal", "paycode", "pos withdrawal", "atm withdrawal", "atm wdl", "cash withdrawal", "cash wdl",
		"atm cash", "agent withdrawal",
	}},
	{category: "Bank Fees & Charges", classification: "Bank Charge", flags: MerchantRule{IsFee: true}, names: []string{
		"account maintenance", "acct maintenance", "maintenance charge", "transfer commission", "nip commission",
		"commission on", "withdrawal fee", "atm fee", "sms notification", "card issuance", "card replacement",
		"statement charge", "cheque book", "fx conversion fee", "international transaction fee", "late payment fee",
	}},
}

// brandConfidence is above the 0.95 a counterparty name needs to decide a transfer on its own.
const brandConfidence = 0.96

func init() {
	// Brands slot in ahead of the generic purpose words ("mama", "rent", "chop") so "Mama Cass" is
	// a restaurant, and they are matched longest name first so "jumia food" wins over "jumia".
	var brands []MerchantRule
	for _, g := range brandGroups {
		for _, name := range g.names {
			rule := g.flags
			rule.Patterns = []string{name}
			rule.Merchant = titleCase(name)
			rule.Category = g.category
			rule.Classification = g.classification
			rule.Confidence = brandConfidence
			rule.WholeWord = true
			rule.CardOnly = cardOnlyBrands[name]
			brands = append(brands, rule)
		}
	}
	sort.SliceStable(brands, func(i, j int) bool { return len(brands[i].Patterns[0]) > len(brands[j].Patterns[0]) })
	MerchantRegistry = append(MerchantRegistry, brands...)
}
