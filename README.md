# Azaman Engine (Shazam for Bank Statements)

> **Ephemeral, sub-second financial intelligence engine designed to decode cryptic Nigerian banking memos, spotlight silent cash leaks (Midnight food/rides, Urgent 2k, duplicate debits, clearinghouse levies), and compute runway — with strict zero-disk persistence.**

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![FastAPI](https://img.shields.io/badge/Laya_AI-FastAPI-009688?style=flat&logo=fastapi)](https://fastapi.tiangolo.com/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![RAM Mode](https://img.shields.io/badge/RAM_Policy-Strictly_Ephemeral-critical)](#zero-disk-ephemeral-ram-architecture)

---

## 1. Executive Summary & Benchmarks

Nigerian bank statements (OPay, PalmPay, GTBank, Kuda, Moniepoint, Access, Zenith, First Bank, UBA, Wema/ALAT, Fidelity, Sterling, FCMB, Stanbic IBTC, Geegpay) are notoriously noisy — filled with clearinghouse settlement routing codes (`NIP/DR/`, `SENT 01655551`), 20-digit transaction session IDs, and truncated narrations.

**Azaman Engine** acts as a *"Shazam for Financial Statements"*:
- Takes any statement (PDF, CSV, Excel, TSV) directly into memory.
- Strips clearinghouse noise, masks PII, and identifies merchants in sub-microsecond time.
- Employs coordinate-geometry table parsing to reconstruct multi-line PDF tables without horizontal text collapse.
- Categorizes spending, surfaces duplicate charges, isolates late-night spending, and measures peer transfer leaks.
- Zeroes out every buffer upon response completion.

### Real-World Performance Benchmarks

| Statement Dataset | Pages / Rows | Processing Time | Memory Persisted | Accuracy / Leaks Identified |
| :--- | :--- | :--- | :--- | :--- |
| **OPay Annual Statement (PDF)** | **559 pages (9,931 txs)** | **4.2 seconds** | **0 bytes** | ₦4.12M leaks (820 Urgent 2k, 196 duplicate debits) |
| **OPay Export (CSV)** | **146 transactions** | **8 milliseconds** | **0 bytes** | Chowdeck double-charge (₦5,148) flagged; OWealth savings & self-transfers excluded from burn |
| **Geegpay USD Statement (CSV)** | **12 transactions** | **2 milliseconds** | **0 bytes** | AWS, GitHub, Cursor classified as Tax Deductibles |
| **Single Memo Shazam (`/decode-memo`)** | **1 transaction** | **0.05 milliseconds** | **0 bytes** | Cleans noise, extracts merchant, tags category |

---

## 2. Zero-Disk Ephemeral RAM Architecture

Azaman enforces strict bank-grade privacy compliance:
1. **Multipart RAM Bounds**: `http.MaxBytesReader` caps every request body at 32MB ([`internal/api/middleware.go`](internal/api/middleware.go)). Gin's `MaxMultipartMemory` is set 1MB *above* that cap ([`internal/api/router.go`](internal/api/router.go)), so multipart parts can never spill to OS temp files.
2. **Zero Disk Writes**: No statement payload, uploaded chunk, or intermediate text ever touches the file system (`os.WriteFile`, `ioutil.TempFile`, or database).
3. **Memory Scrubbing**: When the HTTP handler completes, [`internal/ephemeral/scrubber.go`](internal/ephemeral/scrubber.go) wipes byte buffers by zeroing them in memory:
   ```go
   for i := range b {
       b[i] = 0
   }
   runtime.GC()
   ```
   Zeroing is best-effort: Go's runtime may have copied data (string conversions, GC moves) before the scrub. The guarantee is *no disk persistence*, not forensic RAM erasure.

### Nigeria Data Protection Act (NDPA) 2023 Alignment

| NDPA principle | How Azaman meets it |
| :--- | :--- |
| Data minimisation | Only the uploaded statement is processed; no account linking, no BVN/NIN requested. |
| Storage limitation | Zero-disk design; buffers are scrubbed when the response completes. Nothing is retained after the request. |
| Integrity & confidentiality | NUBAN account and phone numbers are masked (`0123****89`, `0803****567`) before any output or AI call. |
| Purpose limitation | Data is used only to produce the audit returned to the uploader. |
| Cross-border transfer | The optional Laya sidecar runs locally (127.0.0.1); only masked, noise-stripped memos (max 5 per audit) are sent to it. |

> Deployers remain the data controller: they must still provide a privacy notice, a lawful basis (usually consent), and TLS in transit.

---

## 3. High-Level System Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                        HTTP / REST Client                              │
│         (Interactive Web Dashboard / Swagger UI / Next.js API)        │
└────────────────────────────────────┬───────────────────────────────────┘
                                     │ Multipart (32MB Max RAM)
                                     ▼
┌────────────────────────────────────────────────────────────────────────┐
│                          Azaman Engine (Go)                            │
│                                                                        │
│  ┌───────────────────────┐              ┌───────────────────────────┐  │
│  │ Ephemeral Memory Guard│              │  CORS & Rate Middleware   │  │
│  └───────────┬───────────┘              └─────────────┬─────────────┘  │
│              │                                        │                │
│              ▼                                        ▼                │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                 Parser & Normalizer Router                       │  │
│  │   - Coordinate PDF Extractor (Geometry X/Y Windowing)            │  │
│  │   - Dynamic CSV / Excel Delimiter Splitter                       │  │
│  │   - Bank Auto-Detector (15 NG banks, letterhead-first)          │  │
│  └──────────────────────────────────┬───────────────────────────────┘  │
│                                     │                                  │
│                                     ▼                                  │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                     Shazam Intelligence Core                     │  │
│  │   - NIBSS / Clearinghouse Tag Stripper & PII Masker              │  │
│  │   - Merchant Rules Registry (Tech, Commute, Food, Wealth)        │  │
│  │   - Disambiguation: P2P vs. Commercial Vendor vs. POS Agent      │  │
│  └──────────────────┬─────────────────────────────┬─────────────────┘  │
│                     │                             │                    │
│                     ▼                             ▼                    │
│  ┌────────────────────────────────────┐ ┌───────────────────────────┐  │
│  │        Cash Leak Spotlighter       │ │    Laya AI Sidecar Client │  │
│  │  - Urgent 2k Velocity & Errand     │ │  (System 1 Non-Auto-      │  │
│  │  - Midnight Impulse (22:00-03:59)  │ │   regressive Classifier)  │  │
│  │  - Duplicate Debit Detector (24h)  │ └─────────────┬─────────────┘  │
│  │  - Clearinghouse Levies & EMTL     │               │ HTTP (8001)    │
│  │  - Runway & Burn Projector         │               ▼                │
│  └────────────────────────────────────┘ ┌───────────────────────────┐  │
│                                         │ Python FastAPI Laya       │  │
│                                         │ (Model Router + Cache)    │  │
│                                         └───────────────────────────┘  │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Technical Algorithms Breakdown

### A. Coordinate-Geometry PDF Table Extraction
*Source: [`internal/parser/pdf_parser.go`](internal/parser/pdf_parser.go)*

Standard PDF extractors collapse text horizontally, destroying multi-column tables. Azaman maps text cells into an explicit 2D coordinate plane:

1. **Horizontal Boundaries ($X$-Axis)**:
   - Trans Time: $60 \le X \le 85$
   - Description: $160 \le X \le 280$
   - Debit Column: $295 \le X \le 335$
   - Credit Column: $335 \le X \le 375$
   - Balance Column: $375 \le X \le 415$
   - Transaction Reference: $X \ge 460$

2. **Vertical Midpoint Clustering ($Y$-Axis)**:
   Because narration text wraps across 2–3 rows (e.g. recipient name, bank branch, and notes), transactions are windowed between the midpoints of adjacent date timestamps:
   $$Y_{\text{upper}} = \frac{Y_{\text{curr}} + Y_{\text{prev}}}{2}, \quad Y_{\text{lower}} = \frac{Y_{\text{curr}} + Y_{\text{next}}}{2}$$
   All text inside $[Y_{\text{lower}}, Y_{\text{upper}}]$ is merged into the current transaction, preventing text bleed.

---

### B. The Shazam Normalizer & PII Masker
*Source: [`internal/shazam/cleaner.go`](internal/shazam/cleaner.go)*

Cleans banking noise in $< 1\text{ microsecond}$ using deterministic regex compilation:
- **Prefix Cleaning**: Removes `NIP/DR/`, `NIP/CR/`, `TRF/`, `POS 092102/`, `SENT \d+`, `USSD/`.
- **RRN / Reference Stripping**: Removes 12–25 digit routing references (e.g. `250125140200953325498`).
- **PII Redaction**: Masks account numbers, phone numbers, and customer identifiers (`200****551` or `0803***1234`).
- **Punctuation Compression**: Collapses redundant delimiters (`|`, `---`, extra whitespace).

---

### C. Commercial vs. P2P Disambiguation
*Source: [`internal/shazam/matcher.go`](internal/shazam/matcher.go)*

In Nigeria's informal economy, millions of businesses operate using personal accounts. Naive categorization tags every personal name as "Family & Friends". Azaman uses a 3-layer disambiguation engine:

1. **Agency Banking / POS Identification**:
   Recipients matching `POS`, `Agent`, `Cashout`, or colloquial operator nicknames (e.g. `Bode Man`) are routed to **POS Cashout & Agency**, NOT Family favors.
2. **Commercial Entity Suffix Check**:
   Names containing commercial keywords (`Ventures`, `Enterprises`, `Stores`, `Logistics`, `Pharmacy`, `Farms`, `Tech`, `Saloon`, `Motors`, `Bakery`, `Kitchen`, `Plaza`, `Limited`, `Nig Ltd`) are classified as **Commercial & Business Operating Expenses**.
3. **P2P Micro-Transfers**:
   Recipients without commercial suffixes are classified as **Family & Favors**.

---

### D. Cash Leak Algorithms

#### 1. "Urgent 2k" Micro-Favor Velocity
*Source: [`internal/analyzer/urgent2k.go`](internal/analyzer/urgent2k.go)*
$$\text{Condition} = (\text{Debit}) \land (\text{NGN}) \land (1,000 \le \text{Amount} \le 15,000) \land (\text{Category} = \text{"Family \& Favors"})$$
- Tracks weekly velocity: $\text{Transfers / Week} = \frac{\text{Count}}{\text{Total Days} / 7}$.
- Flags recurring micro-transfers that silently erode monthly cash reserves.

#### 2. Midnight Discretionary Drain
*Source: [`internal/analyzer/midnight.go`](internal/analyzer/midnight.go)*
$$\text{Condition} = (\text{Debit}) \land (\text{Hour} \in \{22, 23, 0, 1, 2, 3\}) \land (\text{Discretionary Category})$$
- Window is 22:00–03:59. Filters for late-night impulse spending (fast food, rides, lounges, betting).
- Date-only statements (no time column) parse as 00:00; those rows are treated as *time unknown*, not midnight. Known late-night merchants (Chowdeck, Bolt, lounges) are only assumed late-night when the statement has no time of day.
- Savings deposits (OWealth auto-save etc.) are never midnight leaks.

#### 3. 24-Hour Duplicate Debit Detection
*Source: [`internal/analyzer/duplicates.go`](internal/analyzer/duplicates.go)*
$$\text{Match} \iff (\text{Merchant}_A = \text{Merchant}_B) \land (|A_{\text{amount}} - B_{\text{amount}}| < 0.01) \land (|T_A - T_B| \le 1440\text{ minutes})$$
- Detects POS double-swipes, gateway retry duplications, and automated system double-charges.
- Assigns group IDs (`DUP-001`, `DUP-002`) and computes potential refund recovery.

#### 4. Runway & Burn Rate Projection
*Source: [`internal/analyzer/runway.go`](internal/analyzer/runway.go)*
$$\text{Daily Burn} = \frac{\sum \text{Real Outflows}}{\text{Audit Span in Days}}, \quad \text{Monthly Burn} = \text{Daily Burn} \times 30.44$$
$$\text{Weighted Burn} = \frac{\sum_{k} 0.85^{k} \cdot \text{Burn}_{\text{month}-k}}{\sum_{k} 0.85^{k}} \quad (\text{complete calendar months only})$$
$$\text{Runway (Months)} = \frac{\text{Closing Balance}}{\text{Weighted Burn}}$$
- **Real outflows** exclude self-transfers, savings-wallet deposits and reversed debits.
- **Inflation weighting**: the most recent month weighs 1.0, the one before 0.85, and so on, so a 12-month statement reflects current naira prices rather than last year's.
- **Closing balance** is the balance on the newest-dated row (statements can be newest-first).

---

### E. Laya AI Decision Pass (Hybrid AI Sidecar)
*Source: [`internal/ai/laya_client.go`](internal/ai/laya_client.go) & [`sidecar/laya_service.py`](sidecar/laya_service.py)*

For ambiguous memos where confidence $< 0.90$:
- **Deduplication**: Memos are deduplicated by clean text before querying Laya.
- **Batch Capping**: Capped at $\le 5$ items per statement audit to guarantee sub-second completion.
- **Sidecar Caching**: An in-memory cache in Python stores classified memos, allowing repeat items to return in $0.01\text{ ms}$.
- **Offline Fallback**: If Laya is offline, Go automatically falls back to rule-based heuristics without blocking requests.

### F. Nigeria-Specific Intelligence
*Source: [`internal/analyzer/transfers.go`](internal/analyzer/transfers.go), [`internal/analyzer/insights.go`](internal/analyzer/insights.go), [`internal/analyzer/runway.go`](internal/analyzer/runway.go)*

These passes run before the leak detectors, so leak totals only count real spending.

1. **Failed-Transfer Reversals**: A credit mentioning `reversal`/`refund`/`rvsl` is paired with the closest earlier debit of the same amount within 7 days. Both legs are dropped from spend, income and duplicates. Reversals slower than the **CBN 48-hour** timeline are flagged `late`. Debits marked `failed`/`pending` with no reversal are listed as **dispute candidates**.
2. **Self-Transfers & Savings Wallets**: Transfers to the holder's own name at another bank (at least two shared name tokens, so a sibling sharing only a surname does not count), `Add Money` card top-ups, and OWealth / PiggyVest / Cowrywise / Kuda Save movements are treated as internal. They are reported as savings and removed from burn and leaks.
3. **Urgent 2k Periodicity Filter**: Three or more payments to the same recipient with a stable amount (±10%) at a regular interval (at least 6 days, gap CV ≤ 0.35) are planned obligations, not impulse favours. They are reclassified as **Black Tax (Family Support)**, **Ajo / Thrift**, **Rent** or **Recurring Payment**.
4. **Bank Charges & Overcharge Detector**: Fees are split into EMTL, stamp duty, SMS, USSD, VAT, cybersecurity levy (0.5%), card maintenance, account maintenance and NIP transfer fees. Potential overcharges are flagged for dispute:

   | Rule | Limit |
   | :--- | :--- |
   | EMTL | ₦50 flat, only on receipts ≥ ₦10,000 (levies beyond the qualifying receipt count are flagged) |
   | NIP transfer fee | ₦50 top tier + 7.5% VAT = ₦53.75 |
   | Naira debit card maintenance | ₦50 per quarter |

   USSD fees are totalled but not checked: the ₦6.98 NCC rate is per 20-second session, and one transaction can span several sessions.
5. **Nigerian Categories**: Betting (SportyBet, Bet9ja, BetKing, 1xBet…), loan apps (Palmcredit, Okash, QuickCheck…; loan disbursements are never counted as income), Ajo/Esusu/Adashe, religious giving (tithe, offering, zakat), DSTV/GOtv/streaming, school fees/WAEC/JAMB, rent, electricity DisCos, and bill gateways (CoralPay, Remita, Quickteller). Pidgin memos such as `t fare`, `chop money`, `abeg`, `sapa` and `pikin` are recognised. Short patterns (≤ 4 chars) match whole words only, so `glo` no longer matches "global" and `mobil` no longer turns "Mobile Data" into fuel. FairMoney, Carbon and Renmoney are also banks, so only loan-specific memos from them count as loans.
6. **Insights & Warnings**: Reports betting spend vs winnings, airtime/data share, loan debt-service ratio (a warning above 30%), black tax share and savings rate (net savings + ajo ÷ income).
7. **Tax Estimate (Nigeria Tax Act 2025)**: Annualised real income minus tax-deductible spend and rent relief (20% of rent, capped at ₦500k), taxed with the 2026 PIT bands: 0% to ₦800k, 15% to ₦3M, 18% to ₦12M, 21% to ₦25M, 23% to ₦50M, and 25% above. This is an indicative figure only.
8. **Business vs Personal Split**: Separates tech vendors, commercial payees (`Ventures`, `Enterprises`, `Ent`, `Ltd`…) and client income from personal flows. This gives informal traders who run their business through a personal account a simple P&L.

---

## 5. Repository Structure

```
azaman-engine/
├── cmd/
│   └── server/
│       └── main.go                 # Application bootstrap & graceful shutdown
├── internal/
│   ├── ai/
│   │   ├── laya_client.go          # Bounded Go client for Laya decision sidecar
│   │   └── laya_client_test.go     # Client unit tests
│   ├── analyzer/
│   │   ├── analyzer_test.go        # Audit & leak tests
│   │   ├── duplicates.go           # 24h duplicate debit detector
│   │   ├── leaks.go                # Central cash leak aggregator
│   │   ├── midnight.go             # 10 PM - 4 AM impulse drain detector
│   │   ├── insights.go             # Nigeria insights, NTA 2025 tax estimate, business split
│   │   ├── nigeria_test.go         # Reversal, self-transfer, recurring, overcharge, tax tests
│   │   ├── runway.go               # Weighted burn, runway, bank charges & overcharges
│   │   ├── transfers.go            # Reversals, self/savings transfers, recurring payments
│   │   └── urgent2k.go             # Urgent 2k micro-transfer tracker
│   ├── api/
│   │   ├── api_test.go             # HTTP endpoint tests
│   │   ├── dashboard.go            # Interactive HTML5/JS drag-and-drop UI
│   │   ├── docs.go                 # Swagger UI & OpenAPI 3.0 specification
│   │   ├── handlers.go             # Audit upload & memo decoder handlers
│   │   ├── middleware.go           # 32MB RAM limiter, CORS, request logger
│   │   └── router.go               # Gin route definitions
│   ├── ephemeral/
│   │   └── scrubber.go             # Byte buffer zeroing & GC invoker
│   ├── models/
│   │   ├── audit.go                # Audit report, KPI, and leak models
│   │   └── transaction.go          # Raw and Decoded transaction structs
│   ├── parser/
│   │   ├── csv_parser.go           # Dynamic CSV/TSV/Excel in-memory parser
│   │   ├── detector.go             # Bank auto-detector (OPay, GTB, Kuda, etc.)
│   │   ├── normalizer.go           # Naira/currency sanitizers & date layouts
│   │   ├── parser_test.go          # Parser unit tests
│   │   └── pdf_parser.go           # Column-aware coordinate PDF table parser
│   └── shazam/
│       ├── cleaner.go              # Clearinghouse noise & PII stripper
│       ├── dictionary.go           # Curated Nigerian & global merchant registry
│       ├── matcher.go              # Shazam decoder & P2P disambiguator
│       └── shazam_test.go          # Shazam decoder unit tests
├── sidecar/
│   ├── laya_service.py             # FastAPI server with Laya router & memo cache
│   └── requirements.txt            # Python dependencies (laya, fastapi, uvicorn)
├── testdata/                       # Sample test statements (OPay, GTB, Kuda, Geegpay)
├── .air.toml                       # Live-reload configuration
├── go.mod                          # Go module dependencies
└── README.md                       # Comprehensive technical documentation
```

---

## 6. API Reference

### 1. Statement Audit (Ephemerally In RAM)
`POST /api/v1/shazam/audit`

- **Content-Type**: `multipart/form-data`
- **Parameters**:
  - `statement` (File, required): Bank statement (PDF, CSV, XLSX, TSV).
  - `password` (String, optional): Password for encrypted PDFs.
  - `bank` (String, optional): Bank name override.
  - `account_name` (String, optional): Account holder name. Auto-read from CSV/Excel headers (`Account Name`, `Account Holder`, `Customer Name`); used to spot transfers to your own accounts.
  - `detail` (String, optional): `summary` drops every per-transaction array for a much smaller payload (low-end phones, expensive data). Also accepted as `?detail=summary`.

**Sample Response**:
```json
{
  "summary": {
    "total_inflow": 71115501.04,
    "total_outflow": 16485987.51,
    "net_retained": 54629513.53,
    "profit_margin_pct": 76.82,
    "currency": "NGN",
    "transaction_count": 9931,
    "period_start": "2025-01-25",
    "period_end": "2026-09-24",
    "avg_monthly_burn": 814256.38,
    "runway_months": 0.4
  },
  "cash_leaks": {
    "total_leak_burn": 4122227.64,
    "total_leak_percentage_of_outflow": 25.0,
    "urgent_2k": {
      "total_amount": 3272926.15,
      "count": 820,
      "frequency_per_week": 9.45,
      "average_transfer_size": 3991.37
    },
    "midnight_leak": {
      "total_amount": 684262.00,
      "count": 160,
      "top_merchants": [
        {"merchant": "OPay OWealth Savings & Stash", "total_spent": 593313.50, "count": 140},
        {"merchant": "Chowdeck Delivery", "total_spent": 45648.50, "count": 11}
      ]
    },
    "duplicate_debits": {
      "total_potential_refund": 132036.59,
      "count": 196
    },
    "bank_charges": {
      "total_charges": 33002.90,
      "transfer_fees": 25015.40,
      "stamp_duty": 7880.00,
      "emtl": 7880.00,
      "vat_on_fees": 0,
      "cybersecurity_levy": 0,
      "total_overcharge": 50.00,
      "overcharges": [
        {"fee_type": "EMTL", "charged": 50, "allowed": 0, "excess": 50, "rule": "EMTL (₦50) applies only to receipts of ₦10,000+; more levies than qualifying receipts"}
      ]
    }
  },
  "reversals": {"reversed_count": 1, "reversed_amount": 20000, "late_reversals": 1, "unresolved_count": 0},
  "internal_transfers": {"self_transfers_out": 1050, "self_transfers_in": 21000, "savings_deposits": 26868, "savings_withdrawals": 29891, "net_saved": -3023},
  "nigeria_insights": {
    "betting_spend": 1000, "airtime_data_spend": 4160, "debt_service_ratio_pct": 0,
    "religious_giving": 60000, "black_tax": 0, "savings_rate_pct": -2.92,
    "recurring_payments": [],
    "warnings": ["Betting: ₦1,000 staked across 1 bets (net -₦1,000).", "Savings rate is -2.9%; aim for at least 10%."]
  },
  "tax_estimate": {"regime": "Nigeria Tax Act 2025 (PIT, effective 2026)", "annualized_income": 63159.27, "estimated_annual_tax": 0},
  "business_split": {"business_inflow": 0, "business_outflow": 5500, "personal_outflow": 145383},
  "categories": [
    {"category": "FinTech & Investments", "amount": 8855265.07, "percentage": 53.71, "count": 1681},
    {"category": "Family & Favors", "amount": 6493771.95, "percentage": 39.39, "count": 920}
  ],
  "audit_meta": {
    "session_id": "55b09d2b-e7f5-4b1b-95f2-af196b0b930e",
    "bank_detected": "OPay",
    "execution_time_ms": 4286,
    "memory_scrubbed": true,
    "file_format": "pdf",
    "file_size": 6374645
  }
}
```

### 2. Decode Single Memo (Sub-Millisecond)
`POST /api/v1/shazam/decode-memo`

**Request**:
```json
{
  "memo": "TRF 093201/CHOWDECK_LAGOS_NG/UBAS_00",
  "amount": 8500,
  "date": "2025-02-15 23:45:00",
  "currency": "NGN"
}
```
**Response**:
```json
{
  "id": "e89d1234-5678-4321-9876-abcdef012345",
  "merchant": "Chowdeck Delivery",
  "category": "Late Night Food & Lounges",
  "classification": "Personal Discretionary",
  "confidence": 0.99,
  "time_window": "Late Night (Discretionary)",
  "flags": {
    "is_midnight_leak": true,
    "is_urgent_2k": false,
    "is_tax_deductible": false
  }
}
```

### 3. Additional Endpoints
- `GET /health`: Diagnostics, uptime, Laya sidecar status, and RAM allocations.
- `GET /swagger`: Interactive OpenAPI 3.0 documentation.
- `GET /`: Interactive web dashboard for drag-and-drop statement audits.

---

## 7. Getting Started

### Prerequisites
- **Go**: Version `1.26+` installed
- **Python**: Version `3.10+` installed (for Laya sidecar)
- **Air** (Optional for live-reload): `go install github.com/air-verse/air@latest`

### Step 1: Clone and Build Engine
```bash
git clone <your-fork-url>/azaman-engine.git
cd azaman-engine
go mod tidy
go build -o bin/server.exe ./cmd/server
```

### Step 2: Start the Laya AI Sidecar (Optional)
```bash
pip install -r sidecar/requirements.txt
# run from the repo root so the `sidecar.` module path resolves
python -m uvicorn sidecar.laya_service:app --host 127.0.0.1 --port 8001
```

### Step 3: Run the Engine
With live-reloading:
```bash
air
```
Or directly:
```bash
./bin/server.exe
```

Access the UI at:
- Dashboard: [http://localhost:8080](http://localhost:8080)
- Swagger API: [http://localhost:8080/swagger](http://localhost:8080/swagger)
- Health: [http://localhost:8080/health](http://localhost:8080/health)

### Step 4: Run Tests
```bash
go test -v ./...
```
