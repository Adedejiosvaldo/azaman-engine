import sys
import time
from typing import List, Dict, Any, Optional
from fastapi import FastAPI
from pydantic import BaseModel

app = FastAPI(title="Azaman Laya Decision Sidecar", version="1.0.0")

# Attempt to load native Laya Router
HAS_LAYA = False
router = None

try:
    from laya import Router
    print("[LAYA-SIDECAR] Loading Laya Router checkpoints into memory...")
    router = Router(preload=True)
    HAS_LAYA = True
    print("[LAYA-SIDECAR] Laya Router successfully preloaded!")
except Exception as e:
    print(f"[LAYA-SIDECAR] Laya package not loaded ({e}). Running in high-speed fallback mode.")
    HAS_LAYA = False


class MemoItem(BaseModel):
    id: str
    memo: str
    amount: float
    raw_memo: Optional[str] = ""


class BatchRequest(BaseModel):
    items: List[MemoItem]


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
        "criteria": {
            "Food & Dining": "restaurants, fast food, meals, eating out, bakeries",
            "Supermarket & Groceries": "supermarkets, grocery stores, provisions, household shopping",
            "Commute & Transit": "bus transit, taxis, fuel, petrol stations, ride hailing",
            "POS Cashout & Agency": "POS merchants, cashout agents, business enterprise withdrawals",
            "Tech & Hardware": "laptops, software, cloud hosting, developer tools, subscriptions",
            "Airtime & Utilities": "mobile data, telecom airtime, electricity, power utility bills",
            "Family & Favors": "personal transfers to individuals, allowances, upkeep, gifts",
            "Religious & Giving": "church tithes, offerings, donations",
            "Bank Fees & Levies": "stamp duty, EMTL, USSD fees, alert charges, transfer fees",
            "Personal Discretionary": "general lifestyle spending, shopping, leisure"
        }
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


@app.get("/health")
def health():
    return {
        "status": "ok",
        "model": "laya-typed-decisions" if HAS_LAYA else "fallback-classifier",
        "engine": "Laya AI System 1 Decision Model",
        "is_native_laya": HAS_LAYA
    }


memo_cache: Dict[str, Dict[str, Any]] = {}


@app.post("/api/v1/laya/classify-batch")
def classify_batch(req: BatchRequest) -> Dict[str, Any]:
    start_time = time.time()
    results: List[DecisionResult] = []

    for item in req.items:
        text = item.memo
        if not text and item.raw_memo:
            text = item.raw_memo

        cache_key = text.strip().lower()
        if cache_key in memo_cache:
            cached = memo_cache[cache_key]
            results.append(DecisionResult(
                id=item.id,
                merchant=cached["merchant"],
                category=cached["category"],
                classification=cached["classification"],
                confidence=cached["confidence"],
                is_silent_leak=cached["is_silent_leak"],
                is_urgent_2k=cached["is_urgent_2k"],
                is_tax_deductible=cached["is_tax_deductible"]
            ))
            continue

        if HAS_LAYA and router is not None:
            # Predict using Laya non-autoregressive decision pass
            state = {"body": text, "amount": item.amount}
            pred = router.predict(state, CATEGORY_QUESTIONS)
            answers = pred.get("answers", {})

            cat_choice = answers.get("category", {}).get("choice", "Personal Discretionary")
            cat_conf = answers.get("category", {}).get("answer_confidence", 0.90)

            is_leak_prob = answers.get("is_silent_leak", {}).get("noul", 0.0)
            is_u2k_prob = answers.get("is_urgent_2k", {}).get("noul", 0.0)
            is_tax_prob = answers.get("is_tax_deductible", {}).get("noul", 0.0)

            classification = "Personal Discretionary"
            if is_tax_prob > 0.6:
                classification = "Tax-Deductible Business Expense"
            elif is_u2k_prob > 0.6:
                classification = "Micro-Transfer (Urgent 2k)"

            m_title = clean_title(text)
            dec = DecisionResult(
                id=item.id,
                merchant=m_title,
                category=cat_choice,
                classification=classification,
                confidence=round(cat_conf, 3),
                is_silent_leak=is_leak_prob > 0.5,
                is_urgent_2k=is_u2k_prob > 0.5,
                is_tax_deductible=is_tax_prob > 0.5
            )
            memo_cache[cache_key] = {
                "merchant": m_title,
                "category": cat_choice,
                "classification": classification,
                "confidence": round(cat_conf, 3),
                "is_silent_leak": is_leak_prob > 0.5,
                "is_urgent_2k": is_u2k_prob > 0.5,
                "is_tax_deductible": is_tax_prob > 0.5
            }
            results.append(dec)
        else:
            # High-speed semantic heuristic fallback
            lower = text.lower()
            cat = "Personal Discretionary"
            classification = "Personal Discretionary"
            is_tax = False
            is_leak = False
            is_u2k = False
            conf = 0.88

            if "laptop" in lower or "macbook" in lower or "hardware" in lower or "tech" in lower:
                cat = "Tech & Hardware"
                classification = "Tax-Deductible Business Expense"
                is_tax = True
                conf = 0.95
            elif "transport" in lower or "transit" in lower or "bus" in lower:
                cat = "Commute & Transit"
                classification = "Tax-Deductible Business Expense"
                is_tax = True
                conf = 0.93
            elif "enterprise" in lower or "monie point" in lower or "pos" in lower:
                cat = "POS Cashout & Agency"
                conf = 0.90
            elif "food" in lower or "eat" in lower or "bread" in lower or "corn" in lower:
                cat = "Food & Dining"
                is_leak = True
                conf = 0.92
            elif "allowance" in lower or "feeding" in lower or "upkeep" in lower:
                cat = "Family & Favors"
                classification = "Micro-Transfer (Urgent 2k)"
                is_u2k = True
                conf = 0.94

            if item.amount <= 15000 and ("transfer" in lower or "opay" in lower or "palmpay" in lower):
                if cat == "Personal Discretionary" or cat == "Family & Favors":
                    is_u2k = True
                    classification = "Micro-Transfer (Urgent 2k)"

            results.append(DecisionResult(
                id=item.id,
                merchant=clean_title(text),
                category=cat,
                classification=classification,
                confidence=conf,
                is_silent_leak=is_leak,
                is_urgent_2k=is_u2k,
                is_tax_deductible=is_tax
            ))

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
