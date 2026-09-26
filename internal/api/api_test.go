package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"azaman-engine/internal/models"
)

func TestHealthEndpoint(t *testing.T) {
	router := SetupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}
}

func TestMerchantsEndpoint(t *testing.T) {
	router := SetupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/merchants", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}

	total, ok := resp["total_merchants"].(float64)
	if !ok || total <= 0 {
		t.Errorf("expected total_merchants > 0, got %v", resp["total_merchants"])
	}
}

func TestDecodeMemoEndpoint(t *testing.T) {
	router := SetupRouter()

	payload := map[string]interface{}{
		"memo":   "TRF 093201/CHOWDECK_LAGOS_NG/UBAS_00",
		"amount": 8500,
		"date":   "2024-01-15 23:15:00",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/shazam/decode-memo", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status  string                   `json:"status"`
		Decoded models.DecodedTransaction `json:"decoded"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Decoded.Merchant != "Chowdeck Delivery" {
		t.Errorf("expected Chowdeck Delivery, got %s", resp.Decoded.Merchant)
	}
	if !resp.Decoded.Flags.IsMidnightLeak {
		t.Errorf("expected IsMidnightLeak true")
	}
}

func TestAuditGTBankStatement(t *testing.T) {
	router := SetupRouter()

	filePath := filepath.Join("..", "..", "testdata", "gtbank_statement.csv")
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("statement", "gtbank_statement.csv")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	part.Write(fileContent)
	writer.Close()

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/shazam/audit", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var report models.AuditReport
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatalf("failed to parse audit report: %v", err)
	}

	// Assertions on GTBank audit report
	if report.BankDetected != "GTBank" {
		t.Errorf("expected GTBank, got %s", report.BankDetected)
	}
	if report.Summary.TotalInflow != 2500000.0 {
		t.Errorf("expected 2,500,000 inflow, got %f", report.Summary.TotalInflow)
	}
	if report.CashLeaks.MidnightLeak.Count == 0 {
		t.Errorf("expected midnight leaks to be found")
	}
	if report.CashLeaks.Urgent2k.Count == 0 {
		t.Errorf("expected urgent 2k transactions to be found")
	}
	if report.CashLeaks.DuplicateDebits.Count == 0 {
		t.Errorf("expected duplicate debits to be found (Choplife)")
	}
	if report.CashLeaks.DuplicateDebits.TotalPotentialRefund != 15000.0 {
		t.Errorf("expected 15,000 refund, got %f", report.CashLeaks.DuplicateDebits.TotalPotentialRefund)
	}
	if report.CashLeaks.BankCharges.TotalCharges <= 0 {
		t.Errorf("expected bank charges to be identified")
	}
	if !report.AuditMeta.MemoryScrubbed {
		t.Errorf("expected memory scrubbed true")
	}
}

func TestAuditGeegpayUSDStatement(t *testing.T) {
	router := SetupRouter()

	filePath := filepath.Join("..", "..", "testdata", "geegpay_usd_statement.csv")
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("statement", "geegpay_usd_statement.csv")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	part.Write(fileContent)
	writer.Close()

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/shazam/audit", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var report models.AuditReport
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatalf("failed to parse audit report: %v", err)
	}

	if report.BankDetected != "Geegpay USD" {
		t.Errorf("expected Geegpay USD, got %s", report.BankDetected)
	}
	if report.Summary.Currency != "USD" {
		t.Errorf("expected USD currency, got %s", report.Summary.Currency)
	}
	if report.Summary.TotalInflow != 4500.0 {
		t.Errorf("expected 4500 USD inflow, got %f", report.Summary.TotalInflow)
	}

	// Verify tax deductible tech classification
	hasTaxDeductible := false
	for _, tx := range report.Transactions {
		if tx.Flags.IsTaxDeductible {
			hasTaxDeductible = true
			break
		}
	}
	if !hasTaxDeductible {
		t.Errorf("expected SaaS expenses to be flagged as tax deductible")
	}
}

func TestAuditOPayStatement(t *testing.T) {
	router := SetupRouter()

	filePath := filepath.Join("..", "..", "testdata", "opay_statement.csv")
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read OPay test file: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("statement", "opay_statement.csv")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	part.Write(fileContent)
	writer.Close()

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/shazam/audit", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var report models.AuditReport
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatalf("failed to parse OPay audit report: %v", err)
	}

	if report.BankDetected != "OPay" {
		t.Errorf("expected OPay bank detected, got %s", report.BankDetected)
	}

	if len(report.Transactions) == 0 {
		t.Fatalf("expected transactions to be parsed from OPay statement, got 0")
	}

	t.Logf("Parsed %d OPay transactions in %d ms", len(report.Transactions), report.AuditMeta.ExecutionTimeMs)
	t.Logf("Summary: Inflow=₦%f, Outflow=₦%f, Net=₦%f, Margin=%.1f%%, Runway=%.1f months",
		report.Summary.TotalInflow,
		report.Summary.TotalOutflow,
		report.Summary.NetRetained,
		report.Summary.ProfitMarginPct,
		report.Summary.RunwayMonths,
	)
	t.Logf("Leaks: Midnight=%d txs (₦%f), Urgent2k=%d txs (₦%f), Duplicates=%d pairs (₦%f refund), BankCharges=₦%f",
		report.CashLeaks.MidnightLeak.Count,
		report.CashLeaks.MidnightLeak.TotalAmount,
		report.CashLeaks.Urgent2k.Count,
		report.CashLeaks.Urgent2k.TotalAmount,
		report.CashLeaks.DuplicateDebits.Count,
		report.CashLeaks.DuplicateDebits.TotalPotentialRefund,
		report.CashLeaks.BankCharges.TotalCharges,
	)

	if report.Summary.TotalOutflow <= 0 {
		t.Errorf("expected positive total outflow")
	}
	if report.CashLeaks.BankCharges.TotalCharges <= 0 {
		t.Errorf("expected EMTL / USSD bank charges to be detected")
	}
}
