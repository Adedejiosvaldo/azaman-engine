"""Builds testdata/memo_wide.jsonl: a second, wider labelled set for scoring the categoriser.

Run:  python -I testdata/memo_wide_build.py
Score: go run ./cmd/memo-eval -data testdata/memo_wide.jsonl -split test

memo_eval.jsonl checks the hard cases (businesses and purposes named in transfers). This set
checks breadth: well-known Nigerian merchants, billers and bank system rows, in the many ways
statements print them. It was written before the rules were widened; tune against the "dev"
half only, and report the "test" half.

People's names and account numbers are invented. Merchants are real brands.
"""

import json
import random
from pathlib import Path

random.seed(11)
rows = []
BANKS = ["OPay", "PalmPay", "MONIE POINT", "Kuda MFB", "Access Bank", "GTBank", "Zenith Bank", "UBA", "First Bank"]


def num(n):
    return "".join(random.choice("0123456789") for _ in range(n))


def phone():
    return random.choice(["0803", "0806", "0703", "0813", "0902", "0705", "0811"]) + num(7)


def pos(m):
    return random.choice([
        f"POS DEBIT: {num(11)} {m} LAGOS NG",
        f"POS/WEB PURCHASE TRANSACTION -{num(6)}- {m} LA LANG",
        f"POS Purchase {m} {num(8)}",
        f"CARD PURCHASE {m} NG {num(6)}",
    ])


def web(m):
    return random.choice([
        f"WEB PURCHASE: PAYSTACK *{m}",
        f"WEB PMT {m.upper()} {num(10)}",
        f"FLW*{m} LAGOS NG",
        f"PSTK/{m}/{num(9)}",
        f"Card Payment | {m}",
    ])


def transfer(name, purpose=None):
    f = random.choice(["pipe", "nip", "dash", "trf"])
    if f == "pipe":
        s = f"Transfer to {name} | {random.choice(BANKS)} | {num(10)}"
        return s + (f" | {purpose}" if purpose else "")
    if f == "nip":
        return f"NIP/DR/{num(10)}/{name}" + (f"/{purpose}" if purpose else "")
    if f == "dash":
        return f"Transfer to {name}" + (f" - {purpose}" if purpose else "")
    return f"TRF/{purpose or 'Transfer'}/FRM TUNDE BAKARE TO {name}"


def add(memo, want, kind, alt=(), amount=None):
    rows.append({"memo": memo, "amount": amount or random.choice([500, 1200, 2500, 4000, 7500, 12000, 25000, 60000]),
                 "hour": None, "want": want, "alt": list(alt), "kind": kind})


def each(items, want, kind, wrap, alt=()):
    for m in items:
        add(wrap(m), want, kind, alt)


FOOD, NIGHT, MOVE, UTIL, TV = "Food & Dining", "Late Night Food & Lounges", "Commute & Transit", "Airtime & Utilities", "Subscriptions & TV"
TECH, BET, LOAN, INVEST, FEES = "Tech & Cloud Infrastructure", "Betting & Gaming", "Loans & Debt Service", "FinTech & Investments", "Bank Fees & Charges"
SCHOOL, RENT, FAITH, CARE, SHOP = "Education", "Rent & Housing", "Religious Giving", "Personal Care", "Shopping & Retail"
HEALTH, CASH, AJO, FAMILY, BILLS = "Health & Medical", "POS Cashout & Agency", "Ajo & Thrift", "Family & Favors", "Bills & Online Payments"
FUN, VENDOR, BIZ, UNKNOWN = "Personal Discretionary", "Market & Small Vendors", "Commercial & Business", "Uncategorized"

# ---- Card and web payments to well-known merchants ----
each(["CHICKEN REPUBLIC IKEJA", "KILIMANJARO RESTAURANT", "MR BIGGS", "TANTALIZERS", "SWEET SENSATION", "THE PLACE LEKKI",
      "ITEM 7 GO", "MEGA CHICKEN", "KFC ADMIRALTY", "BURGER KING VI", "PIZZA HUT", "DEBONAIRS PIZZA", "SHOPRITE IKEJA",
      "SPAR NIGERIA", "JUSTRITE SUPERSTORE", "EBEANO SUPERMARKET", "MARKET SQUARE", "HUBMART STORES", "FOODCO",
      "BLENCO SUPERMARKET", "ADDIDE SUPERMARKET", "CAFE ONE", "ART CAFE", "MAMA CASS", "SOOYAH BISTRO"], FOOD, "merchant", pos, alt=[NIGHT])
each(["QUILOX NIGHT CLUB", "CUBANA LOUNGE", "THE HOUSE BAR", "BHEERHUGZ CAFE", "SHISHA ROOM", "W BAR LOUNGE", "CLUB 57"], NIGHT, "merchant", pos)
each(["Chowdeck", "Glovo", "Jumia Food", "HEYFOOD", "MANO"], NIGHT, "merchant", web, alt=[FOOD])
each(["UBER *TRIP", "BOLT.EU/R/2609", "INDRIVE", "RIDA NG", "LAGRIDE", "GOKADA", "GIGM.COM", "ABC TRANSPORT", "PEACE MASS TRANSIT",
      "GUO TRANSPORT", "COWRY CARD TOPUP", "LCC TOLL LEKKI", "AIR PEACE", "ARIK AIR", "IBOM AIR", "UNITED NIGERIA AIRLINES",
      "GREEN AFRICA AIRWAYS", "WAKANOW", "TRAVELSTART"], MOVE, "merchant", web, alt=[FUN])
each(["NNPC MEGA STATION", "TOTALENERGIES IKOYI", "MOBIL FILLING STATION", "OANDO SERVICE STATION", "ARDOVA PLC AP", "MRS OIL",
      "CONOIL STATION", "RAINOIL LTD", "ENYO RETAIL", "NIPCO GAS", "BOVAS PETROLEUM", "NORTHWEST PETROLEUM"], MOVE, "merchant", pos)
each(["DSTV", "GOTV", "STARTIMES", "SHOWMAX", "NETFLIX.COM", "SPOTIFY AB", "APPLE.COM/BILL", "YOUTUBE PREMIUM", "GOOGLE *YouTube",
      "AMAZON PRIME VIDEO", "BOOMPLAY", "AUDIOMACK", "DISNEY PLUS", "GOOGLE ONE", "ICLOUD STORAGE"], TV, "merchant", web, alt=[TECH])
each(["AMAZON WEB SERVICES", "GOOGLE CLOUD", "DIGITALOCEAN.COM", "VERCEL INC", "GITHUB INC", "NAMECHEAP.COM", "WHOGOHOST",
      "HOSTINGER", "CLOUDFLARE", "OPENAI *CHATGPT", "ANTHROPIC CLAUDE", "CURSOR AI", "FIGMA INC", "NOTION LABS", "SLACK TECHNOLOGIES",
      "ZOOM.US", "HEROKU", "RENDER.COM", "NETLIFY", "JETBRAINS", "MICROSOFT 365", "ADOBE CREATIVE CLOUD", "CANVA PTY", "GODADDY"],
     TECH, "merchant", web, alt=[TV])
each(["BET9JA", "SPORTYBET", "1XBET", "BETKING", "BETWAY", "NAIRABET", "MSPORT", "22BET", "BETWINNER", "BABA IJEBU", "PARIPESA",
      "ILOTBET", "BETANO", "FOOTBALL.COM", "LIVESCORE BET", "MERRYBET"], BET, "merchant", web)
each(["FAIRMONEY", "CARBON LOAN REPAYMENT", "BRANCH INTERNATIONAL", "PALMCREDIT", "RENMONEY", "OKASH", "EASEMONI", "AELLA CREDIT",
      "QUICKCHECK", "SPECTA", "NEWCREDIT", "XCREDIT", "LCREDIT", "PAGE FINANCIALS", "ZEDVANCE", "CREDIT DIRECT", "LAPO MICROFINANCE LOAN",
      "KREDIBANK"], LOAN, "merchant", web)
each(["PIGGYVEST", "COWRYWISE", "RISEVEST", "BAMBOO INVEST", "TROVE FINANCE", "CHAKA", "BINANCE", "BYBIT", "QUIDAX", "LUNO",
      "BUSHA", "YELLOW CARD", "ARM INVESTMENT", "STANBIC IBTC ASSET MGT", "MERISTEM", "INVESTNOW", "CHAPEL HILL DENHAM"], INVEST, "merchant", web)
each(["JUMIA", "KONGA.COM", "TEMU.COM", "SHEIN", "ALIEXPRESS", "AMAZON.COM", "SLOT SYSTEMS", "POINTEK", "3CHUB", "PEP STORES",
      "MR PRICE", "MINISO", "ZARA", "NIKE STORE", "JIJI.NG", "KARA.COM.NG", "FOUANI NIGERIA", "ISTORE NG", "SAMSUNG EXPERIENCE STORE",
      "BALOGUN MARKET FABRICS", "RUFF N TUMBLE", "CASABELLA", "SUPER SAVERS HOME"], SHOP, "merchant", pos, alt=[FUN])
each(["MEDPLUS PHARMACY", "HEALTHPLUS LTD", "ALPHA PHARMACY", "REDDINGTON HOSPITAL", "LAGOON HOSPITALS", "EVERCARE HOSPITAL",
      "ST NICHOLAS HOSPITAL", "SYNLAB NIGERIA", "CLINA LANCET LABORATORIES", "RELIANCE HMO", "HYGEIA HMO", "AVON HMO",
      "DENTAL CLINIC IKEJA", "EYE FOUNDATION HOSPITAL", "MECURE HEALTHCARE", "NETT PHARMACY", "FAMILY CARE CLINIC"], HEALTH, "merchant", pos, alt=[SHOP])
each(["ROYAL TOUCH UNISEX SALON", "KUTS BARBING", "GLAM NAILS STUDIO", "ORIKI SPA", "ZARON COSMETICS", "HOUSE OF TARA",
      "I-FITNESS GYM", "BODYLINE FITNESS", "GARMENT CARE DRY CLEANERS", "WASHMAN LAUNDRY"], CARE, "merchant", pos, alt=[FUN])
each(["FILMHOUSE CINEMAS", "GENESIS CINEMAS", "SILVERBIRD CINEMAS", "PLAYSTATION NETWORK", "STEAM GAMES", "LANDMARK BEACH",
      "OMU RESORT", "FUNTASTICLAND", "TICKETMASTER", "TIX AFRICA", "RADISSON BLU HOTEL", "EKO HOTELS", "BOOKING.COM", "AIRBNB"],
     FUN, "merchant", web, alt=[SHOP, MOVE])
each(["REMITA", "QUICKTELLER", "BAXI", "INTERSWITCH BILLS", "FIRS TAX PAYMENT", "LIRS", "NIMC NIN", "IMMIGRATION PASSPORT",
      "FRSC DRIVERS LICENCE", "LEADWAY ASSURANCE", "AXA MANSARD", "AIICO INSURANCE", "LASG REVENUE", "CAC REGISTRATION"], BILLS, "merchant", web, alt=[UNKNOWN])
each(["WAEC", "JAMB", "NECO", "ULESSON", "COURSERA", "UDEMY", "ALTSCHOOL AFRICA", "UNILAG SCHOOL FEES", "COVENANT UNIVERSITY",
      "GREENSPRINGS SCHOOL", "CORONA SCHOOLS", "DUOLINGO", "BRITISH COUNCIL IELTS", "NIIT"], SCHOOL, "merchant", web)
each(["RCCG", "LIVING FAITH CHURCH", "MFM MINISTRIES", "DEEPER LIFE", "CHRIST EMBASSY", "DAYSTAR CHRISTIAN CENTRE",
      "HOUSE ON THE ROCK", "NASFAT", "ANSAR UD DEEN", "CATHOLIC ARCHDIOCESE", "ANGLICAN COMMUNION", "COZA", "ELEVATION CHURCH"],
     FAITH, "business", transfer)
each(["IKEDC", "EKEDC PREPAID", "AEDC", "IBEDC", "PHED", "KEDCO", "EEDC", "JED", "BEDC", "IKEJA ELECTRIC", "EKO ELECTRICITY",
      "SPECTRANET", "SMILE COMMUNICATIONS", "FIBERONE", "IPNX", "STARLINK", "TIZETI WIFI", "MTN NIGERIA", "AIRTEL NETWORKS",
      "GLO MOBILE", "9MOBILE", "BUYPOWER.NG"], UTIL, "merchant", web)

# ---- Rows the bank or wallet writes itself ----
for memo, want, alt in [
    (f"Mobile Data | {phone()} | MTN", UTIL, []), (f"Airtime | {phone()} | Airtel", UTIL, []), (f"Airtime | {phone()} | GLO", UTIL, []),
    (f"Electricity | IKEDC | {num(11)}", UTIL, []), (f"TV | DSTV | {num(10)}", TV, []), (f"TV | GOtv | {num(10)}", TV, []),
    (f"Betting | SportyBet | {num(8)}", BET, []), (f"Betting | Bet9ja | {num(8)}", BET, []),
    ("Electronic Money Transfer Levy", FEES, []), ("Stamp Duty", FEES, []), ("USSD Charge", FEES, []), ("VAT on Transfer Fee", FEES, []),
    ("SMS ALERT CHARGE JAN 2026", FEES, []), ("CARD MAINTENANCE FEE", FEES, []), ("NIP TRANSFER COMMISSION", FEES, []),
    ("ACCOUNT MAINTENANCE CHARGE", FEES, []), ("ATM WITHDRAWAL FEE OTHER BANK", FEES, [CASH]), ("VAT ON COMMISSION", FEES, []),
    ("SafeBox Deposit", INVEST, []), ("Spend & Save Deposit", INVEST, []), ("Auto-save to OWealth Balance", INVEST, ["Internal Transfers"]),
    ("Fixed Deposit Placement", INVEST, []), ("Target Savings Auto Debit", INVEST, []), ("Kuda Save pocket top up", INVEST, []),
    (f"AIRTIME PURCHASE {phone()} MTN VTU", UTIL, []), (f"DATA BUNDLE PURCHASE {phone()}", UTIL, []), (f"MTN VTU {phone()} USSD", UTIL, []),
    (f"AIRTIME/GLO/{phone()}", UTIL, []), (f"*737*{num(4)}# AIRTIME TOPUP", UTIL, []), (f"PREPAID METER TOKEN {num(11)}", UTIL, []),
    (f"ATM WDL {num(8)} ALLEN AVENUE LAGOS", CASH, []), (f"ATM CASH WITHDRAWAL @ {num(8)} GTB IKEJA", CASH, []),
    (f"POS WITHDRAWAL {num(8)} AGENT", CASH, []), (f"Cash Withdrawal | POS | {num(10)}", CASH, []), ("CARDLESS WITHDRAWAL PAYCODE", CASH, []),
    (f"Loan Repayment | {num(10)}", LOAN, []), ("OKASH AUTO REPAYMENT", LOAN, []), ("OVERDRAFT INTEREST CHARGE", LOAN, [FEES]),
    (f"Third-Party Merchant Order | JUMIA | {num(12)}", SHOP, []), (f"Third-Party Merchant Order | SPORTYBET | {num(12)}", BET, [SHOP]),
    (f"Third-Party Merchant Order | BOLT | {num(12)}", MOVE, [SHOP]),
]:
    add(memo, want, "system", alt)

# ---- Transfers with a typed purpose ----
for name, purpose, want, alt in [
    ("KEMI ADEYEMI", "school fees second term", SCHOOL, []), ("SEUN OKAFOR", "house rent 2026", RENT, []),
    ("CHIDI NWOSU", "caution fee and agency", RENT, []), ("AMINA BELLO", "tithe", FAITH, []), ("GRACE EZE", "offering and first fruit", FAITH, []),
    ("BOLA ADENIYI", "ajo contribution week 4", AJO, []), ("NGOZI OBI", "esusu", AJO, []), ("MUSA IBRAHIM", "cooperative dues", AJO, []),
    ("FEMI ALABI", "hospital bill", HEALTH, []), ("TOLU AJAYI", "drugs and malaria test", HEALTH, []), ("EMEKA UDEH", "dental checkup", HEALTH, []),
    ("SADE BALOGUN", "hair and nails", CARE, []), ("KUNLE OJO", "barbing", CARE, []), ("IFE ADEBAYO", "laundry", CARE, [FUN]),
    ("YEMI OLADELE", "fuel", MOVE, []), ("DAYO SALAMI", "transport fare", MOVE, []), ("UCHE OKORO", "uber to airport", MOVE, []),
    ("HALIMA SANI", "foodstuff for the month", FOOD, []), ("TUNDE AJIBOLA", "lunch", FOOD, []), ("BISI OLATUNJI", "shawarma and drinks", FOOD, [NIGHT]),
    ("JOHN OKEKE", "data subscription", UTIL, []), ("MARY UMEH", "nepa bill", UTIL, []), ("PETER OBASI", "light token", UTIL, []),
    ("ESTHER DANJUMA", "dstv subscription", TV, []), ("SAMUEL ADEKUNLE", "netflix", TV, []),
    ("RUTH AKPAN", "upkeep for mummy", FAMILY, ["Black Tax (Family Support)"]), ("DAVID ETIM", "pocket money", FAMILY, []),
    ("JOY NWACHUKWU", "birthday gift", FAMILY, [FUN]), ("PAUL OGBONNA", "feeding allowance", FAMILY, [FOOD]),
    ("LARA COKER", "loan repayment", LOAN, []), ("BEN ONYEKA", "refund of money borrowed", LOAN, [FAMILY]),
    ("ZAINAB YUSUF", "aso ebi", SHOP, [FUN]), ("KOLA ADEWALE", "shoes and bag", SHOP, []), ("NKEM OBIORA", "phone screen repair", SHOP, [FUN]),
    ("TAIWO OGUNLEYE", "website hosting renewal", TECH, []), ("SEGUN BAMIDELE", "lesson teacher", SCHOOL, []),
    ("FUNKE AKINDELE", "wedding contribution", FAMILY, [FUN]), ("OBINNA EZEH", "mechanic workmanship", MOVE, [VENDOR, FUN]),
    ("ADA NNAMDI", "waste bill lawma", RENT, [UTIL]), ("ISAAC OLAWALE", "estate service charge", RENT, []),
    ("MERCY ATANDA", "gym membership", CARE, [FUN]), ("VICTOR UGWU", "betting", BET, []), ("ROSE IKENNA", "cinema tickets", FUN, []),
]:
    add(transfer(name, purpose), want, "purpose", alt)

# ---- Businesses paid by transfer: the name says what was bought ----
for name, want, alt in [
    ("ALHAJA KUDI FOODSTUFF STORE", FOOD, [VENDOR]), ("IYA IBEJI AMALA SPOT", FOOD, []), ("DE CHOICE FAST FOOD", FOOD, []),
    ("BLESSED HANDS PHARMACY", HEALTH, [SHOP]), ("GOOD SHEPHERD MEDICAL CENTRE", HEALTH, []), ("PRIME DIAGNOSTICS LABORATORY", HEALTH, []),
    ("FIRST CLASS OPTICALS", HEALTH, [SHOP]), ("EMMANUEL AUTO MECHANIC WORKS", MOVE, [VENDOR, BIZ]), ("JOE TYRES AND SPARE PARTS", MOVE, [VENDOR, BIZ, SHOP]),
    ("GLORY LAND NURSERY AND PRIMARY SCHOOL", SCHOOL, []), ("BRIGHT FUTURE TUTORIAL CENTRE", SCHOOL, []),
    ("DIVINE FAVOUR HAIR PALACE", CARE, []), ("SMOOTH CUTS BARBERSHOP", CARE, []), ("OGA TAILOR FASHION HOME", SHOP, [VENDOR]),
    ("COMPUTER VILLAGE GADGET HUB", SHOP, []), ("EMEKA BUILDING MATERIALS", SHOP, [BIZ, VENDOR, RENT]), ("SUNSHINE HOTEL AND SUITES", FUN, []),
    ("VICTORY GAS PLANT", UTIL, [MOVE, VENDOR]), ("ROYAL PRINTING PRESS", BIZ, [VENDOR]), ("CHUKS LOGISTICS SERVICES", MOVE, [BIZ]),
    ("LAGOS STATE WATER CORPORATION", RENT, [UTIL, BILLS]), ("ZENITH GENERAL INSURANCE", BILLS, [BIZ]),
    ("ADEX GLOBAL VENTURES", BIZ, [VENDOR]), ("TOPMOST ENTERPRISES NIG LTD", BIZ, [VENDOR]),
]:
    add(transfer(name), want, "business", alt)

# ---- Nothing to go on: must not be given a confident wrong label ----
for memo in ["Debit adjustment", f"TRF/{num(12)}/REF", "Charge reversal correction", f"WEB PMT {num(14)}", "MISC DEBIT", "Payment",
             f"NIP/DR/{num(10)}/TRANSFER", "Online transaction", "QR Payment", f"REF:{num(16)}"]:
    add(memo, UNKNOWN, "unknown", [FUN, BILLS])

random.shuffle(rows)
for i, r in enumerate(rows):
    r["split"] = "dev" if i % 2 == 0 else "test"

out = Path(__file__).with_name("memo_wide.jsonl")
out.write_text("\n".join(json.dumps(r) for r in rows) + "\n", encoding="utf-8")
print(len(rows), "rows ->", out.name)
