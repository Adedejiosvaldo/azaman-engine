"""Small memo classifier trained on your own labelled transfer memos.

Laya is a general model and reads Nigerian transfer memos poorly ("Curtains" -> Ajo & Thrift).
This model learns from memos you label: character n-grams catch pidgin, misspellings and
abbreviations ("tf", "t fare", "trnsprt"), and logistic regression gives probabilities that
mean something, so the engine can tell a sure answer from a guess.

    python sidecar/memo_model.py export tmp/audit.json sidecar/data/labels.csv
    # fill in the "label" column, then:
    python sidecar/memo_model.py train sidecar/data/labels.csv

Labels and the trained model hold personal memo text: they live in sidecar/data and
sidecar/models, which are git-ignored.
"""
import csv
import json
import re
import sys
from pathlib import Path
from typing import Dict, List, Optional, Tuple

HERE = Path(__file__).parent
MODEL_PATH = HERE / "models" / "memo_model.joblib"
CATEGORIES = [c["name"] for c in json.loads((HERE / "categories.json").read_text("utf-8"))["categories"] if c["assignable"]]

# A label must be seen this often before the model may answer with it.
MIN_PER_LABEL = 3

_digits = re.compile(r"\d+")
_space = re.compile(r"\s+")


def normalise(text: str) -> str:
    """Lower-case and collapse numbers so "2 bags" and "3 bags" read the same."""
    return _space.sub(" ", _digits.sub("0", (text or "").lower())).strip()


# ==================== EXPORT ====================
def export(audit_path: str, out_path: str) -> int:
    """Write the unique memos the engine would ask AI about, one row each, ready to label.

    "suggested" is the engine's own answer; copy it into "label" when it is right. Rows already
    in out_path keep their labels, so exporting more statements grows the file.
    """
    report = json.loads(Path(audit_path).read_text("utf-8"))
    existing = _read_labels(out_path) if Path(out_path).exists() else {}

    rows: Dict[str, dict] = {}
    for tx in report.get("transactions", []):
        flags = tx.get("flags", {})
        if not tx.get("debit") or any(flags.get(f) for f in ("is_bank_fee", "is_savings", "is_loan", "is_self_transfer")):
            continue
        # The same text the engine sends Laya: the typed purpose, or the memo when there is no payee
        text = tx.get("purpose") or ("" if tx.get("counterparty") else tx.get("clean_memo", ""))
        key = normalise(text)
        if not _has_word(key) or key in rows:
            continue
        rows[key] = {"memo": text, "amount": tx["debit"], "suggested": tx.get("category") if tx.get("category") in CATEGORIES else "",
                     "label": existing.get(key, "")}

    Path(out_path).parent.mkdir(parents=True, exist_ok=True)
    with open(out_path, "w", newline="", encoding="utf-8") as fh:
        w = csv.DictWriter(fh, fieldnames=["memo", "amount", "suggested", "label"])
        w.writeheader()
        for key in sorted(rows, key=lambda k: (rows[k]["label"] != "", k)):  # unlabelled first
            w.writerow(rows[key])
        # keep labelled memos from earlier statements that are not in this one
        for key, label in existing.items():
            if key not in rows:
                w.writerow({"memo": key, "amount": "", "suggested": "", "label": label})
    return len(rows)


_presets = {"purchase", "personal transfer", "transfer", "payment"}


def _has_word(text: str) -> bool:
    """Account numbers and OPay presets ("8160166645 Purchase") carry nothing to learn from."""
    words = [w for w in re.findall(r"[a-z]{2,}", text)]
    return bool(words) and " ".join(words) not in _presets


def _read_labels(path: str) -> Dict[str, str]:
    with open(path, newline="", encoding="utf-8") as fh:
        return {normalise(r["memo"]): r["label"].strip() for r in csv.DictReader(fh) if r.get("label", "").strip()}


# ==================== TRAIN ====================
def build_pipeline():
    from sklearn.feature_extraction.text import TfidfVectorizer
    from sklearn.linear_model import LogisticRegression
    from sklearn.pipeline import FeatureUnion, make_pipeline

    features = FeatureUnion([
        ("chars", TfidfVectorizer(analyzer="char_wb", ngram_range=(2, 5), sublinear_tf=True, min_df=1)),
        ("words", TfidfVectorizer(analyzer="word", ngram_range=(1, 2), sublinear_tf=True, token_pattern=r"(?u)\b\w+\b")),
    ])
    return make_pipeline(features, LogisticRegression(max_iter=2000, C=5.0, class_weight="balanced"))


def train(labels_path: str, model_path: Path = MODEL_PATH) -> dict:
    """Train on the labelled rows, report cross-validated accuracy, and save the model."""
    import joblib
    from sklearn.model_selection import StratifiedKFold, cross_val_predict

    labels = _read_labels(labels_path)
    unknown = sorted({l for l in labels.values() if l not in CATEGORIES})
    if unknown:
        raise SystemExit(f"Unknown labels {unknown}. Use one of:\n  " + "\n  ".join(CATEGORIES))

    counts: Dict[str, int] = {}
    for label in labels.values():
        counts[label] = counts.get(label, 0) + 1
    kept = {m: l for m, l in labels.items() if counts[l] >= MIN_PER_LABEL}
    dropped = sorted(l for l, n in counts.items() if n < MIN_PER_LABEL)
    if len(set(kept.values())) < 2:
        raise SystemExit(f"Need at least 2 labels with {MIN_PER_LABEL}+ examples each; have {counts}")

    texts, ys = list(kept), list(kept.values())
    folds = min(5, min(counts[l] for l in set(ys)))
    predicted = cross_val_predict(build_pipeline(), texts, ys, cv=StratifiedKFold(folds, shuffle=True, random_state=0))
    accuracy = sum(p == y for p, y in zip(predicted, ys)) / len(ys)

    model = build_pipeline().fit(texts, ys)
    model_path.parent.mkdir(parents=True, exist_ok=True)
    joblib.dump({"pipeline": model, "accuracy": accuracy, "examples": len(ys)}, model_path)

    per_label = {}
    for label in sorted(set(ys)):
        idx = [i for i, y in enumerate(ys) if y == label]
        per_label[label] = (sum(predicted[i] == label for i in idx), len(idx))
    return {"examples": len(ys), "accuracy": accuracy, "per_label": per_label, "dropped": dropped,
            "model": str(model_path)}


# ==================== SERVE ====================
class MemoModel:
    def __init__(self, pipeline, accuracy: float):
        self.pipeline, self.accuracy = pipeline, accuracy

    def predict(self, text: str) -> Tuple[str, float]:
        """Best category and its probability."""
        probs = self.pipeline.predict_proba([normalise(text)])[0]
        best = probs.argmax()
        return str(self.pipeline.classes_[best]), float(probs[best])


def load(model_path: Path = MODEL_PATH) -> Optional[MemoModel]:
    if not model_path.exists():
        return None
    import joblib
    saved = joblib.load(model_path)
    return MemoModel(saved["pipeline"], saved["accuracy"])


def main(argv: List[str]) -> None:
    if len(argv) == 3 and argv[0] == "export":
        n = export(argv[1], argv[2])
        print(f"Wrote {n} memos to {argv[2]}. Fill the 'label' column with one of:\n  " + "\n  ".join(CATEGORIES))
    elif len(argv) == 2 and argv[0] == "train":
        r = train(argv[1])
        print(f"Trained on {r['examples']} labelled memos -> {r['model']}")
        print(f"Cross-validated accuracy: {r['accuracy']:.0%}")
        for label, (right, total) in r["per_label"].items():
            print(f"  {right:>3}/{total:<3} {label}")
        if r["dropped"]:
            print(f"Not learned yet (fewer than {MIN_PER_LABEL} examples): {', '.join(r['dropped'])}")
    else:
        print(__doc__)
        sys.exit(2)


if __name__ == "__main__":
    main(sys.argv[1:])
