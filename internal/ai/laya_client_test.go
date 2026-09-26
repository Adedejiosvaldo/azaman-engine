package ai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLayaClient_Offline(t *testing.T) {
	client := &LayaClient{
		BaseURL:    "http://127.0.0.1:59999", // dead port
		httpClient: &http.Client{Timeout: 100},
	}

	if client.IsAvailable() {
		t.Errorf("expected IsAvailable to be false for dead port")
	}
}

func TestLayaClient_MockSidecar(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}

		if r.URL.Path == "/api/v1/laya/classify-batch" {
			resp := batchResponse{
				ExecutionTimeMs: 15.2,
				Count:           2,
				Results: []LayaDecision{
					{
						ID:              "tx-1",
						Merchant:        "Addide Supermarket",
						Category:        "Supermarket & Groceries",
						Classification:  "Essential Living",
						Confidence:      0.94,
						IsSilentLeak:    false,
						IsUrgent2k:      false,
						IsTaxDeductible: false,
					},
					{
						ID:              "tx-2",
						Merchant:        "P2P Favor",
						Category:        "Family & Favors",
						Classification:  "Micro-Transfer (Urgent 2k)",
						Confidence:      0.92,
						IsSilentLeak:    false,
						IsUrgent2k:      true,
						IsTaxDeductible: false,
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &LayaClient{
		BaseURL:    server.URL,
		httpClient: server.Client(),
	}

	if !client.IsAvailable() {
		t.Errorf("expected mock server to be available")
	}

	items := []LayaMemoItem{
		{ID: "tx-1", Memo: "Transfer to ADDIDE IKORODU", Amount: 2400},
		{ID: "tx-2", Memo: "Transfer to Mum for feeding", Amount: 2000},
	}

	decisions, err := client.ClassifyBatch(items)
	if err != nil {
		t.Fatalf("ClassifyBatch failed: %v", err)
	}

	if len(decisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(decisions))
	}

	d1, ok := decisions["tx-1"]
	if !ok || d1.Category != "Supermarket & Groceries" {
		t.Errorf("expected Supermarket & Groceries, got %v", d1)
	}

	d2, ok := decisions["tx-2"]
	if !ok || !d2.IsUrgent2k {
		t.Errorf("expected IsUrgent2k = true for tx-2, got %v", d2)
	}
}
