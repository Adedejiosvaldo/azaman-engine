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

func TestAuditMultipleStatements(t *testing.T) {
	router := SetupRouter()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for _, name := range []string{"gtbank_statement.csv", "kuda_statement.csv"} {
		content, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		part, err := writer.CreateFormFile("statements", name)
		if err != nil {
			t.Fatalf("failed to create form file: %v", err)
		}
		part.Write(content)
	}
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
	if len(report.Accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(report.Accounts))
	}
	if report.Accounts[0].Label != "GTBank" || report.Accounts[1].Label != "Kuda Bank" {
		t.Errorf("unexpected labels: %q, %q", report.Accounts[0].Label, report.Accounts[1].Label)
	}
	total := 0
	for _, a := range report.Accounts {
		total += a.Transactions
	}
	if total != len(report.Transactions) {
		t.Errorf("account row counts (%d) should add up to all transactions (%d)", total, len(report.Transactions))
	}
	for _, tx := range report.Transactions {
		if tx.Account == "" {
			t.Fatalf("every transaction should carry its account label")
		}
	}
}

func TestAuditSingleStatementHasOneAccount(t *testing.T) {
	router := SetupRouter()
	content, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "opay_statement.csv"))
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("statement", "opay_statement.csv")
	part.Write(content)
	writer.Close()

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/shazam/audit", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var report models.AuditReport
	json.Unmarshal(w.Body.Bytes(), &report)
	if len(report.Accounts) != 1 || report.Accounts[0].Label != report.BankDetected {
		t.Errorf("single upload should report one account labelled by bank, got %+v", report.Accounts)
	}
}

func TestEventsOnlyAcceptKnownNames(t *testing.T) {
	router := SetupRouter()
	for name, want := range map[string]int{"grade_shared": http.StatusNoContent, "my_bank_password": http.StatusBadRequest} {
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewBufferString(`{"event":"`+name+`"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != want {
			t.Errorf("event %q: expected %d, got %d", name, want, w.Code)
		}
	}
}

func TestStatsNeedsToken(t *testing.T) {
	router := SetupRouter()
	t.Setenv("STATS_TOKEN", "secret")
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", w.Code)
	}
	req.Header.Set("Authorization", "Bearer secret")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with token, got %d", w.Code)
	}
}
