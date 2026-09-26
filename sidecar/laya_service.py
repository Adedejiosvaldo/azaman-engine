import hashlib
import json
import os
import re
import threading
import time
from collections import OrderedDict
from pathlib import Path
from typing import Any, Dict, List, Optional

from fastapi import FastAPI
from pydantic import BaseModel, Field

app = FastAPI(title="Azaman Laya Decision Sidecar", version="1.1.0")

# ==================== CATEGORY TAXONOMY ====================
# Shared with the Go engine (sidecar/categories.json). Laya may only answer with "assignable" categories,
# so its output always lines up with the engine's insight, fee and business totals.
_CATEGORY_FILE = Path(__file__).with_name("categories.json")
with _CATEGORY_FILE.open(encoding="utf-8") as fh:
    _TAXONOMY = json.load(fh)["categories"]

ASSIGNABLE = {c["name"]: c["description"] for c in _TAXONOMY if c["assignable"]}
DEFAULT_CATEGORY = "Personal Discretionary"
LEAK_CATEGORIES = {"Late Night Food & Lounges", "Betting & Gaming", "Subscriptions & TV"}
TAX_CATEGORIES = {"Tech & Cloud Infrastructure", "Workspace & Co-working"}
URGENT_2K_MIN, URGENT_2K_MAX = 1000.0, 15000.0

# ==================== LAYA ROUTER ====================
HAS_LAYA = False
router = None

if os.getenv("LAYA_DISABLE") != "1":
    try:
        from laya import Router
        print("[LAYA-SIDECAR] Loading Laya Router checkpoints into memory...")
        router = Router(preload=True)
        HAS_LAYA = True
        print("[LAYA-SIDECAR] Laya Router successfully preloaded!")
    except Exception as e:
        print(f"[LAYA-SIDECAR] Laya package not loaded ({e}). Running in high-speed fallback mode.")


# ==================== REQUEST / RESPONSE MODELS ====================
class MemoItem(BaseModel):
    id: str = Field(max_length=64)
    memo: str = Field(max_length=300)
    amount: float = Field(ge=0)
    raw_memo: Optional[str] = Field(default="", max_length=300)
    hour: Optional[int] = Field(default=None, ge=0, le=23)  # time of day, None if the statement has no time
    is_credit: bool = False
    currency: str = Field(default="NGN", max_length=3)


class BatchRequest(BaseModel):
    items: List[MemoItem] = Field(max_length=50)


class DecisionResult(BaseModel):
    id: str
    merchant: str
    category: str
    classification: str
    confidence: float
    is_silent_leak: bool
    is_urgent_2k: bool
    is_tax_deductible: bool


CATEGORY_QUESTIONS = {
    "category": {
        "type": "choice",
        "instructions": "Classify this Nigerian bank transfer or payment memo into the most appropriate category.",
        "criteria": ASSIGNABLE,
    },
    "is_silent_leak": {
        "type": "noul",
        "instructions": "Is this a non-essential discretionary cash leak or late night drain?"
    },
    "is_urgent_2k": {
        "type": "noul",
        "instructions": "Is this an 'urgent 2k' micro-favor or small personal transfer under 15,000 NGN?"
    },
    "is_tax_deductible": {
        "type": "noul",
        "instructions": "Is this a legitimate business expense such as software, coworking, hardware, or corporate commute?"
    }
}


# ==================== EPHEMERAL DECISION CACHE ====================
class DecisionCache:
    """Bounded LRU with TTL. Keys are SHA-256 hashes, and values hold only the decision, so no memo
    text or recipient names stay in memory after a request (NDPA storage limitation)."""

    def __init__(self, max_entries: int = 10_000, ttl_seconds: float = 3600):
        self.max_entries = max_entries
        self.ttl = ttl_seconds
        self._data: "OrderedDict[str, tuple[float, Dict[str, Any]]]" = OrderedDict()
        self._lock = threading.Lock()

    @staticmethod
    def key(item: MemoItem, text: str) -> str:
        # Amount band, late-night window and direction change the answer, so they are part of the key.
        band = "micro" if URGENT_2K_MIN <= item.amount <= URGENT_2K_MAX else "other"
        late = item.hour is not None and (item.hour >= 22 or item.hour <= 3)
        raw = f"{text.strip().lower()}|{band}|{late}|{item.is_credit}|{item.currency}"
        return hashlib.sha256(raw.encode("utf-8")).hexdigest()

    def get(self, k: str) -> Optional[Dict[str, Any]]:
        with self._lock:
            entry = self._data.get(k)
            if entry is None:
                return None
            stored_at, value = entry
            if time.monotonic() - stored_at > self.ttl:
                del self._data[k]
                return None
            self._data.move_to_end(k)
            return value

    def put(self, k: str, value: Dict[str, Any]) -> None:
        with self._lock:
            self._data[k] = (time.monotonic(), value)
            self._data.move_to_end(k)
            while len(self._data) > self.max_entries:
                self._data.popitem(last=False)

    def clear(self) -> int:
        with self._lock:
            n = len(self._data)
            self._data.clear()
            return n

    def __len__(self) -> int:
        return len(self._data)


memo_cache = DecisionCache(
    max_entries=int(os.getenv("LAYA_CACHE_MAX", "10000")),
    ttl_seconds=float(os.getenv("LAYA_CACHE_TTL", "3600")),
)


# ==================== FALLBACK RULES ====================
# Whole-word regexes, checked in order. Mirrors the Go dictionary: "bus" must not hit "business",
# "pos" must not hit "deposit", bank names (OPay, PalmPay, Carbon) never imply a category.
def _rx(words: str) -> re.Pattern:
    return re.compile(rf"\b(?:{words})\b", re.IGNORECASE)


FALLBACK_RULES = [
    (_rx(r"sportybet|bet9ja|betking|1xbet|betway|nairabet|merrybet|msport|betting"), "Betting & Gaming"),
    (_rx(r"loan|okash|palmcredit|quickcheck|easemoni|lcredit"), "Loans & Debt Service"),
    (_rx(r"ajo|esusu|osusu|adashe|thrift|contribution|cooperative"), "Ajo & Thrift"),
    (_rx(r"church|tithe|offering|mosque|zakat|sadaqah|rccg|mfm|winners"), "Religious Giving"),
    (_rx(r"dstv|gotv|startimes|netflix|spotify|showmax"), "Subscriptions & TV"),
    (_rx(r"school fees?|tuition|waec|jamb|neco"), "Education"),
    (_rx(r"rent|house rent|caution fee|agency fee"), "Rent & Housing"),
    (_rx(r"airtime|mobile data|data bundle|recharge|electricity|nepa|phcn|prepaid meter"), "Airtime & Utilities"),
    (_rx(r"laptop|macbook|hosting|software|server|domain"), "Tech & Cloud Infrastructure"),
    (_rx(r"co-?working|workspace|desk rental"), "Workspace & Co-working"),
    (_rx(r"chowdeck|glovo|lounge|club|nightlife"), "Late Night Food & Lounges"),
    (_rx(r"bolt|uber|indrive|keke|okada|danfo|brt|t[- ]?fare|transport(?: fare)?|bus fare|fuel|petrol"), "Commute & Transit"),
    (_rx(r"hair|barbing|barber|makeup|nails"), "Personal Care"),
    (_rx(r"food|chop|bread|rice|soup|foodstuffs?|market|provisions?"), "Food & Dining"),
    (_rx(r"pos|cashout|cash out|agent"), "POS Cashout & Agency"),
    (_rx(r"allowance|upkeep|feeding|mum|mummy|dad|daddy|mama|papa|abeg|urgent|borrow|sapa|pikin|aunty|uncle"), "Family & Favors"),
    (_rx(r"ventures?|enterprises?|stores?|ltd|limited|nig|ent|plc|global|services"), "Commercial & Business"),
]


def fallback_decision(item: MemoItem, text: str) -> Dict[str, Any]:
    category, confidence = DEFAULT_CATEGORY, 0.6  # below Go's heuristics, so Go keeps its answer
    for pattern, cat in FALLBACK_RULES:
        if pattern.search(text):
            category, confidence = cat, 0.9
            break
    return build_decision(item, text, category, confidence)


def build_decision(item: MemoItem, text: str, category: str, confidence: float,
                   leak_prob: Optional[float] = None, u2k_prob: Optional[float] = None,
                   tax_prob: Optional[float] = None) -> Dict[str, Any]:
    if category not in ASSIGNABLE:
        category = DEFAULT_CATEGORY

    late_night = item.hour is not None and (item.hour >= 22 or item.hour <= 3)
    is_leak = category in LEAK_CATEGORIES or (late_night and category in {"Food & Dining", "Commute & Transit"})
    is_u2k = (category == "Family & Favors" and item.currency == "NGN"
              and URGENT_2K_MIN <= item.amount <= URGENT_2K_MAX)
    is_tax = category in TAX_CATEGORIES

    # Laya probabilities, when present, can only add flags the category allows
    if leak_prob is not None:
        is_leak = is_leak or leak_prob > 0.5
    if u2k_prob is not None:
        is_u2k = is_u2k or (u2k_prob > 0.5 and category == "Family & Favors")
    if tax_prob is not None:
        is_tax = is_tax or (tax_prob > 0.6 and category in TAX_CATEGORIES | {"Commercial & Business"})

    if item.is_credit:
        is_leak = is_u2k = is_tax = False

    classification = "Personal Discretionary"
    if is_tax:
        classification = "Tax-Deductible Business Expense"
    elif is_u2k:
        classification = "Micro-Transfer (Urgent 2k)"
    elif category == "Commercial & Business":
        classification = "Operating Expense"

    return {
        "merchant": clean_title(text),
        "category": category,
        "classification": classification,
        "confidence": round(confidence, 3),
        "is_silent_leak": is_leak,
        "is_urgent_2k": is_u2k,
        "is_tax_deductible": is_tax,
    }


def laya_decision(item: MemoItem, text: str) -> Dict[str, Any]:
    state = {"body": text, "amount": item.amount, "currency": item.currency,
             "direction": "credit" if item.is_credit else "debit"}
    if item.hour is not None:
        state["hour"] = item.hour
    pred = router.predict(state, CATEGORY_QUESTIONS)
    answers = pred.get("answers", {})
    return build_decision(
        item, text,
        category=answers.get("category", {}).get("choice", DEFAULT_CATEGORY),
        confidence=answers.get("category", {}).get("answer_confidence", 0.9),
        leak_prob=answers.get("is_silent_leak", {}).get("noul", 0.0),
        u2k_prob=answers.get("is_urgent_2k", {}).get("noul", 0.0),
        tax_prob=answers.get("is_tax_deductible", {}).get("noul", 0.0),
    )


# ==================== ENDPOINTS ====================
@app.get("/health")
def health():
    return {
        "status": "ok",
        "model": "laya-typed-decisions" if HAS_LAYA else "fallback-classifier",
        "engine": "Laya AI System 1 Decision Model",
        "is_native_laya": HAS_LAYA,
        "version": app.version,
        "categories": len(ASSIGNABLE),
        "cache_entries": len(memo_cache),
        "cache_max": memo_cache.max_entries,
        "cache_ttl_seconds": memo_cache.ttl,
    }


@app.delete("/cache")
def clear_cache():
    return {"cleared": memo_cache.clear()}


@app.post("/api/v1/laya/classify-batch")
def classify_batch(req: BatchRequest) -> Dict[str, Any]:
    start_time = time.time()
    results: List[DecisionResult] = []

    for item in req.items:
        text = item.memo or item.raw_memo or ""
        key = DecisionCache.key(item, text)

        decision = memo_cache.get(key)
        if decision is None:
            if HAS_LAYA and router is not None:
                try:
                    decision = laya_decision(item, text)
                except Exception as e:  # never fail the batch because one prediction broke
                    print(f"[LAYA-SIDECAR] predict failed, using fallback: {e}")
                    decision = fallback_decision(item, text)
            else:
                decision = fallback_decision(item, text)
            # Merchant names are per-memo text, so they are not cached
            memo_cache.put(key, {k: v for k, v in decision.items() if k != "merchant"})
        else:
            decision = {**decision, "merchant": clean_title(text)}

        results.append(DecisionResult(id=item.id, **decision))

    elapsed_ms = (time.time() - start_time) * 1000
    return {
        "execution_time_ms": round(elapsed_ms, 2),
        "count": len(results),
        "results": [r.model_dump() for r in results]
    }


def clean_title(s: str) -> str:
    # Extract clean business/person name from transfer strings
    parts = s.split("|")
    candidate = parts[0].replace("Transfer to", "").replace("Transfer from", "").strip()
    if candidate:
        words = candidate.split()
        if len(words) > 4:
            return " ".join(words[:4]).title()
        return candidate.title()
    return s[:30].title()


if __name__ == "__main__":
    import uvicorn
    port = 8001
    print(f"[LAYA-SIDECAR] Starting Laya Decision Model Server on port {port}...")
    uvicorn.run(app, host="127.0.0.1", port=port)
