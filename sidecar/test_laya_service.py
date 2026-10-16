"""Run from repo root: python -m unittest sidecar.test_laya_service -v"""
import os
import time
import unittest

os.environ["LAYA_DISABLE"] = "1"  # test the deterministic fallback, not model checkpoints

from fastapi.testclient import TestClient  # noqa: E402

from sidecar import laya_service as svc  # noqa: E402

client = TestClient(svc.app)


def classify(*items):
    resp = client.post("/api/v1/laya/classify-batch", json={"items": list(items)})
    assert resp.status_code == 200, resp.text
    return {r["id"]: r for r in resp.json()["results"]}


def item(i, memo, amount=5000, **kw):
    return {"id": str(i), "memo": memo, "amount": amount, **kw}


class FallbackRules(unittest.TestCase):
    def setUp(self):
        svc.memo_cache.clear()

    def test_categories_are_engine_categories(self):
        out = classify(item(1, "Transfer to RANDOM PERSON | OPay"), item(2, "Betting | SPORTYBET"),
                       item(3, "tithe for sunday"), item(4, "DSTV compact"))
        for r in out.values():
            self.assertIn(r["category"], svc.ASSIGNABLE)
        self.assertEqual(out["2"]["category"], "Betting & Gaming")
        self.assertEqual(out["3"]["category"], "Religious Giving")
        self.assertTrue(out["4"]["is_silent_leak"])

    def test_substring_false_positives_are_gone(self):
        out = classify(
            item(1, "Deposit for business purpose"),   # pos / bus / eat inside words
            item(2, "Transfer to X | OPay | 7012345678"),  # bank name is not a favor
            item(3, "great wheat"),
        )
        for r in out.values():
            self.assertEqual(r["category"], "Personal Discretionary")
            self.assertFalse(r["is_urgent_2k"])
            self.assertLess(r["confidence"], 0.85)  # Go keeps its own answer

    def test_transport_fare_is_personal_not_tax(self):
        r = classify(item(1, "t fare to work"))["1"]
        self.assertEqual(r["category"], "Commute & Transit")
        self.assertFalse(r["is_tax_deductible"])

    def test_hng_tech_limited_is_business_not_tax_deductible(self):
        r = classify(item(1, "Transfer to HNG TECH LIMITED"))["1"]
        self.assertEqual(r["category"], "Commercial & Business")
        self.assertFalse(r["is_tax_deductible"])

    def test_urgent_2k_needs_family_category_and_micro_amount(self):
        out = classify(item(1, "abeg urgent", 2000), item(2, "abeg urgent", 50000),
                       item(3, "upkeep", 2000, is_credit=True))
        self.assertTrue(out["1"]["is_urgent_2k"])
        self.assertFalse(out["2"]["is_urgent_2k"])
        self.assertFalse(out["3"]["is_urgent_2k"])

    def test_food_is_leak_only_late_at_night(self):
        out = classify(item(1, "food", hour=13), item(2, "food", hour=23))
        self.assertFalse(out["1"]["is_silent_leak"])
        self.assertTrue(out["2"]["is_silent_leak"])


class Cache(unittest.TestCase):
    def setUp(self):
        svc.memo_cache.clear()

    def test_cache_stores_no_plaintext(self):
        classify(item(1, "Transfer to CHIDINMA OKONKWO upkeep"))
        dump = repr(svc.memo_cache._data).lower()
        self.assertNotIn("chidinma", dump)
        self.assertNotIn("okonkwo", dump)

    def test_cache_is_bounded(self):
        cache = svc.DecisionCache(max_entries=3, ttl_seconds=60)
        for i in range(5):
            cache.put(str(i), {"v": i})
        self.assertEqual(len(cache), 3)
        self.assertIsNone(cache.get("0"))
        self.assertIsNotNone(cache.get("4"))

    def test_cache_expires(self):
        cache = svc.DecisionCache(max_entries=3, ttl_seconds=0.01)
        cache.put("k", {"v": 1})
        time.sleep(0.02)
        self.assertIsNone(cache.get("k"))

    def test_cache_hit_keeps_per_memo_merchant(self):
        out1 = classify(item(1, "Transfer to ADA OBI | abeg"))
        out2 = classify(item(2, "Transfer to ADA OBI | abeg"))
        self.assertEqual(out1["1"]["merchant"], out2["2"]["merchant"])
        self.assertEqual(len(svc.memo_cache), 1)

    def test_delete_cache(self):
        classify(item(1, "food"))
        self.assertEqual(client.delete("/cache").json()["cleared"], 1)
        self.assertEqual(client.get("/health").json()["cache_entries"], 0)


class InputLimits(unittest.TestCase):
    def test_rejects_oversized_batch(self):
        resp = client.post("/api/v1/laya/classify-batch", json={"items": [item(i, "x") for i in range(51)]})
        self.assertEqual(resp.status_code, 422)

    def test_rejects_oversized_memo(self):
        resp = client.post("/api/v1/laya/classify-batch", json={"items": [item(1, "x" * 301)]})
        self.assertEqual(resp.status_code, 422)


class LayaConfidenceTest(unittest.TestCase):
    def test_model_confidence_is_scaled_below_go_rules(self):
        class FakeRouter:
            def predict(self, state, questions):
                return {"answers": {"category": {"choice": "Ajo & Thrift", "confidence": 1.0}}}

        old = svc.router
        svc.router = FakeRouter()
        try:
            item = svc.MemoItem(id="1", memo="Curtains", amount=35000)
            d = svc.laya_decision(item, item.memo)
        finally:
            svc.router = old
        self.assertLessEqual(d["confidence"], 0.7)


if __name__ == "__main__":
    unittest.main()
