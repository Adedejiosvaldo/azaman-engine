"""Builds testdata/memo_eval.jsonl: labelled Nigerian bank memos for scoring the categoriser.

Run:  python -I testdata/memo_eval_build.py
Each row: memo (as a bank prints it), amount (naira debit), hour (or null), want (right category),
alt (other acceptable answers), kind (business | purpose | merchant | unknown | person).
"person" rows are plain transfers to individuals: no model can know their purpose, so they are
not scored for accuracy; the test checks the AI never changes them.

Names, businesses and account numbers here are invented. Formats copy real statements.
"""

import json
import random
from pathlib import Path

random.seed(7)

BANKS = ["OPay", "PalmPay", "MONIE POINT", "Kuda MFB", "Access Bank", "GTBank", "Zenith Bank", "UBA", "First Bank"]


def acct():
    return str(random.randint(1000000000, 9999999999))


def pipe(name, purpose=None):
    s = f"Transfer to {name} | {random.choice(BANKS)} | {acct()}"
    return s + (f" | {purpose}" if purpose else "")


def dash(name, purpose):
    return f"Transfer to {name} - {purpose}"


def nip(name, purpose=None):
    s = f"NIP/DR/{acct()}/{name}"
    return s + (f"/{purpose}" if purpose else "")


def pos(merchant):
    return f"POS DEBIT: {random.randint(10000000000, 99999999999)} {merchant} LAGOS NG"


def web(merchant):
    return f"WEB PURCHASE: PAYSTACK *{merchant}"


rows = []


def add(memo, want, amount, kind, alt=(), hour=None):
    rows.append({"memo": memo, "amount": amount, "hour": hour, "want": want, "alt": list(alt), "kind": kind})


# ---- Businesses paid by transfer: the name says what was bought ----
BUSINESS = [
    ("IYA BASIRAT BUKA VENTURES", "Food & Dining", 4500),
    ("TASTY CORNER EATERY LTD", "Food & Dining", 7800),
    ("ABULE EGBA FRESH FOODS ENTERPRISES", "Food & Dining", 23000),
    ("MAMA NKECHI RICE AND BEANS STORES", "Food & Dining", 36000),
    ("GOLDEN CRUST BAKERY LIMITED", "Food & Dining", 3200),
    ("ADUNNI FOODSTUFF VENTURES", "Food & Dining", 41000),
    ("CLUB VELVET LOUNGE LTD", "Late Night Food & Lounges", 58000),
    ("SKYBAR NIGHTLIFE ENTERTAINMENT", "Late Night Food & Lounges", 35000),
    ("CANTEEN SHISHA LOUNGE ENT", "Late Night Food & Lounges", 26000),
    ("GIG LOGISTICS LIMITED", "Commute & Transit", 12500, ["Commercial & Business", "Shopping & Retail"]),
    ("ABC TRANSPORT PLC", "Commute & Transit", 18000),
    ("MRS FILLING STATION LEKKI", "Commute & Transit", 25000),
    ("AYOMIDE PETROLEUM VENTURES", "Commute & Transit", 30000),
    ("CLOUDHOST NIGERIA TECHNOLOGIES LTD", "Tech & Cloud Infrastructure", 45000),
    ("WHOGOHOST LIMITED", "Tech & Cloud Infrastructure", 15000),
    ("SOFTLINK SOFTWARE SOLUTIONS", "Tech & Cloud Infrastructure", 60000),
    ("HUB ONE COWORKING SPACE LTD", "Workspace & Co-working", 50000),
    ("THE DESK WORKSPACE ENTERPRISES", "Workspace & Co-working", 35000),
    ("SPECTRANET INTERNET LIMITED", "Workspace & Co-working", 20000, ["Airtime & Utilities"]),
    ("IKEJA ELECTRIC PLC", "Airtime & Utilities", 15000),
    ("EKEDC PREPAID TOKEN VENDOR", "Airtime & Utilities", 20000),
    ("REMITA PAYMENT SERVICES", "Bills & Online Payments", 9000),
    ("QUICKTELLER BILLS LTD", "Bills & Online Payments", 6500),
    ("MULTICHOICE NIGERIA LIMITED", "Subscriptions & TV", 15700),
    ("STARTIMES MEDIA NIG LTD", "Subscriptions & TV", 5500),
    ("GREENFIELD INTERNATIONAL SCHOOL", "Education", 350000),
    ("BRIGHT FUTURE COLLEGE ACCOUNT", "Education", 180000),
    ("KINGS TUTORIAL CENTRE", "Education", 25000),
    ("LANDMARK PROPERTIES AND ESTATES LTD", "Rent & Housing", 1200000),
    ("HOMELINK FACILITY MANAGEMENT", "Rent & Housing", 45000, ["Commercial & Business"]),
    ("RCCG GLORY TABERNACLE PARISH", "Religious Giving", 20000),
    ("CENTRAL MOSQUE ZAKAT COMMITTEE", "Religious Giving", 15000),
    ("LIVING FAITH CHURCH OTA", "Religious Giving", 10000),
    ("BETLAND SPORTS LIMITED", "Betting & Gaming", 5000),
    ("NAIJA BET GAMING LTD", "Betting & Gaming", 2000),
    ("FAIRMONEY MICROFINANCE BANK", "Loans & Debt Service", 32000),
    ("RENMONEY MFB LIMITED", "Loans & Debt Service", 48000),
    ("IYALOJA THRIFT COOPERATIVE", "Ajo & Thrift", 20000),
    ("STAFF MULTIPURPOSE COOPERATIVE SOCIETY", "Ajo & Thrift", 50000),
    ("ROYAL TOUCH UNISEX SALON", "Personal Care", 8000),
    ("KLASSIC BARBING SALOON", "Personal Care", 3000),
    ("GLAM BY TOYIN MAKEUP STUDIO", "Personal Care", 25000),
    ("SLOT SYSTEMS LIMITED", "Shopping & Retail", 185000),
    ("BODE FASHION HOUSE ENTERPRISES", "Shopping & Retail", 42000),
    ("JUMIA SERVICES NIGERIA", "Shopping & Retail", 27500),
    ("TRENDY SHOES AND BAGS STORES", "Shopping & Retail", 19000),
    ("ALABA ELECTRONICS PLAZA", "Shopping & Retail", 95000),
    ("BODE MAN-PEACE ENTERPRISE", "Commercial & Business", 15000, ["Market & Small Vendors"]),
    ("U STAR DIVINE ENT 2", "Commercial & Business", 8000, ["Market & Small Vendors"]),
    ("HNG TECH LIMITED", "Tech & Cloud Infrastructure", 20000, ["Commercial & Business", "Education"]),
    ("OKON GLOBAL CONCEPTS", "Commercial & Business", 70000),
    ("FAITH AND SONS NIG LTD", "Commercial & Business", 120000),
]
for b in BUSINESS:
    name, want, amt = b[0], b[1], b[2]
    alt = b[3] if len(b) > 3 else []
    add(pipe(name), want, amt, "business", alt)

# ---- Transfers to people with a typed purpose: the purpose decides ----
PURPOSE = [
    ("CHIOMA EZE", "foodstuff for the house", "Food & Dining", 28000, ["Market & Small Vendors"]),
    ("TUNDE ADEYEMI", "pepper and tomatoes", "Food & Dining", 6500, ["Market & Small Vendors"]),
    ("HALIMA SANI", "2 bags of rice", "Food & Dining", 150000, ["Market & Small Vendors"]),
    ("EMEKA OBI", "suya and drinks", "Late Night Food & Lounges", 9000, ["Food & Dining"]),
    ("SEGUN BALOGUN", "keke fare", "Commute & Transit", 1500),
    ("IFEANYI NNAJI", "bolt ride refund", "Commute & Transit", 4200),
    ("MUSA IBRAHIM", "fuel money", "Commute & Transit", 10000),
    ("ADAEZE OKAFOR", "school fees balance", "Education", 85000),
    ("KEMI LAWAL", "jamb form", "Education", 7700),
    ("OLUMIDE SHOLA", "house rent 2026", "Rent & Housing", 900000),
    ("GRACE ETIM", "caution fee", "Rent & Housing", 150000),
    ("PASTOR JAMES UDO", "tithe", "Religious Giving", 30000),
    ("BLESSING ANI", "thanksgiving offering", "Religious Giving", 10000),
    ("FATIMA BELLO", "ajo contribution", "Ajo & Thrift", 20000),
    ("NGOZI IBE", "esusu week 4", "Ajo & Thrift", 10000),
    ("AMAKA UCHE", "hair making", "Personal Care", 12000),
    ("KUNLE BADMUS", "barbing", "Personal Care", 2000),
    ("YEMI COKER", "gown and shoes", "Shopping & Retail", 35000),
    ("SAMUEL EKPO", "phone screen repair", "Shopping & Retail", 18000, ["Commercial & Business", "Market & Small Vendors"]),
    ("MAMA TOLU", "upkeep for the month", "Family & Favors", 50000),
    ("DAPO ADEBAYO", "urgent 2k", "Family & Favors", 2000),
    ("BISOLA LAWAL", "birthday gift", "Family & Favors", 15000),
    ("UNCLE BAYO", "transport home for mum", "Family & Favors", 20000, ["Commute & Transit"]),
    ("JOY NWOSU", "mtn data", "Airtime & Utilities", 3500),
    ("FEMI ALAO", "nepa light bill", "Airtime & Utilities", 8000),
    ("CHIDI OKEKE", "dstv renewal", "Subscriptions & TV", 15700),
    ("VICTOR ETUK", "loan repayment", "Loans & Debt Service", 40000, ["Family & Favors"]),
    ("TOLANI ADE", "cow leg", "Food & Dining", 7000, ["Market & Small Vendors"]),
    ("RUKAYAT BELLO", "balance for curtains", "Shopping & Retail", 45000, ["Market & Small Vendors", "Rent & Housing"]),
    ("IBRAHIM GARBA", "generator repair", "Commercial & Business", 25000, ["Shopping & Retail", "Market & Small Vendors"]),
    ("ESTHER AKPAN", "laptop charger", "Shopping & Retail", 14000, ["Tech & Cloud Infrastructure"]),
    ("DANIEL OKORO", "website hosting", "Tech & Cloud Infrastructure", 30000),
    ("PAUL ADEWALE", "betting stake", "Betting & Gaming", 5000),
    ("SANDRA OBI", "chicken and turkey", "Food & Dining", 22000, ["Market & Small Vendors"]),
    ("OLAMIDE AJAYI", "pos cash", "POS Cashout & Agency", 10000),
]
for i, p in enumerate(PURPOSE):
    name, purpose, want, amt = p[0], p[1], p[2], p[3]
    alt = p[4] if len(p) > 4 else []
    memo = [pipe(name, purpose), dash(name, purpose), nip(name, purpose)][i % 3]
    add(memo, want, amt, "purpose", alt)

# ---- Card, POS and web payments: the merchant name decides ----
MERCHANT = [
    (pos("CHOPLIFE RESTAURANT"), "Food & Dining", 12000, []),
    (pos("SHOPRITE LEKKI"), "Food & Dining", 34000, ["Shopping & Retail"]),
    (pos("MAMA OLU KITCHEN"), "Food & Dining", 3500, []),
    (pos("SPAR IKEJA CITY MALL"), "Food & Dining", 27000, ["Shopping & Retail"]),
    (pos("HARD ROCK CAFE"), "Late Night Food & Lounges", 45000, ["Food & Dining"], 23),
    (pos("QUILOX NIGHT CLUB"), "Late Night Food & Lounges", 120000, [], 1),
    (pos("TOTAL ENERGIES ADMIRALTY"), "Commute & Transit", 30000, []),
    (pos("CONOIL FILLING STATION"), "Commute & Transit", 20000, []),
    (pos("H AND M PHARMACY"), "Shopping & Retail", 8500, ["Commercial & Business"]),
    (pos("GAME STORES IKEJA"), "Shopping & Retail", 54000, []),
    (pos("BEAUTY HUB SALON"), "Personal Care", 15000, []),
    (web("DOMINOS PIZZA LEKKI"), "Late Night Food & Lounges", 11000, ["Food & Dining"]),
    (web("FLUTTERWAVE KONGA"), "Shopping & Retail", 39000, []),
    (web("GOOGLE WORKSPACE"), "Tech & Cloud Infrastructure", 9000, []),
    (web("NAMECHEAP DOMAIN"), "Tech & Cloud Infrastructure", 12000, []),
    (web("COURSERA"), "Education", 25000, []),
    (web("SHOWMAX AFRICA"), "Subscriptions & TV", 3500, []),
    (web("APPLE MUSIC"), "Subscriptions & TV", 1500, []),
    (web("BET9JA ONLINE"), "Betting & Gaming", 10000, []),
    (web("IKEJA ELECTRIC TOKEN"), "Airtime & Utilities", 10000, []),
    ("Card Payment: Figma Inc Subscription", "Tech & Cloud Infrastructure", 22000, []),
    ("Card Payment: Udemy Online Course", "Education", 15000, []),
    ("Card Payment: Zara Online Store", "Shopping & Retail", 60000, []),
    ("Card Payment: ChatGPT Plus OpenAI", "Tech & Cloud Infrastructure", 30000, ["Subscriptions & TV"]),
    ("Card Payment: YouTube Premium", "Subscriptions & TV", 1700, []),
    ("Airtime Purchase | MTN | 08031234567", "Airtime & Utilities", 2000, []),
    ("Mobile Data | 08051234567 | Glo", "Airtime & Utilities", 3000, []),
    ("USSD Airtime Recharge 9mobile", "Airtime & Utilities", 1000, []),
    ("Electricity | AEDC Prepaid | 45091234567", "Airtime & Utilities", 15000, []),
    ("Betting | SportyBet | 08031112222", "Betting & Gaming", 3000, []),
    ("Cable TV | GOtv Max", "Subscriptions & TV", 8500, []),
    ("POS W/D 093218 LAGOS AIRPORT TERMINAL 2", "POS Cashout & Agency", 20000, []),
    ("ATM WDL GTB IKOYI BRANCH", "POS Cashout & Agency", 40000, []),
    ("PalmPay POS Cashout Agent Oshodi", "POS Cashout & Agency", 10000, []),
    ("Uber Trip Lagos", "Commute & Transit", 5600, []),
    ("Indrive ride payment", "Commute & Transit", 3200, []),
    ("LCC Toll Lekki Plaza", "Commute & Transit", 500, []),
    ("Cowrywise Savings Plan", "Ajo & Thrift", 50000, ["FinTech & Investments"]),
    ("Chowdeck Order 88213", "Late Night Food & Lounges", 9500, ["Food & Dining"], 22),
    ("Glovo food delivery", "Late Night Food & Lounges", 7000, ["Food & Dining"]),
    ("Carbon loan repayment", "Loans & Debt Service", 25000, []),
    ("Okash Loan Repay", "Loans & Debt Service", 18000, []),
    ("LASG Land Use Charge via Remita", "Bills & Online Payments", 35000, ["Rent & Housing"]),
    ("LAWMA waste bill", "Rent & Housing", 4000, ["Airtime & Utilities"]),
    ("Water Corporation bill payment", "Rent & Housing", 6000, ["Airtime & Utilities"]),
]
for m in MERCHANT:
    memo, want, amt, alt = m[0], m[1], m[2], m[3]
    hour = m[4] if len(m) > 4 else None
    add(memo, want, amt, "merchant", alt, hour)

# ---- Memos nobody can place: the honest answer is "Uncategorized" ----
UNKNOWN = [
    ("Payment ref 88213411", 15000),
    ("TRF/REF 003991827/IFO", 7000),
    ("Web transfer 2093383", 12000),
    ("Paycom debit 99381", 5000),
    ("Instant payment out", 3000),
    ("Debit adjustment", 2500),
    ("Merchant payment 00183", 9000),
    ("Online transfer", 20000),
]
for memo, amt in UNKNOWN:
    add(memo, "Uncategorized", amt, "unknown", ["Personal Discretionary"])

# ---- Plain transfers to people, no purpose: not scored, must stay untouched by the AI ----
PEOPLE = ["NGOZI AMAKA OBI", "VICTORIA CHIOMA OKORO", "SAMUEL DAVID EKPO", "HALIMA BISOLA LAWAL",
          "SEUN ADUNNI BALOGUN", "KEMI ADAEZE NWOSU", "BELLO RUKAYAT ASABI", "JOHN CHUKWU",
          "AISHA MOHAMMED", "TOSIN ADEOLA", "PRECIOUS OKON", "IBRAHIM YUSUF"]
for i, name in enumerate(PEOPLE):
    memo = pipe(name) if i % 2 == 0 else nip(name)
    add(memo, "PERSON", random.choice([2000, 5000, 10000, 25000, 50000]), "person")

random.shuffle(rows)
out = Path(__file__).with_name("memo_eval.jsonl")
with out.open("w", encoding="utf-8", newline="\n") as fh:
    for i, r in enumerate(rows):
        r["split"] = "dev" if i % 2 == 0 else "test"
        fh.write(json.dumps(r, ensure_ascii=False) + "\n")

kinds = {}
for r in rows:
    kinds[r["kind"]] = kinds.get(r["kind"], 0) + 1
print(f"wrote {len(rows)} rows to {out.name}: {kinds}")
