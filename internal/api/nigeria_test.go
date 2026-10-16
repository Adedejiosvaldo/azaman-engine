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

func postStatement(t *testing.T, file string, fields map[string]string) (*httptest.ResponseRecorder, models.AuditReport) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", file))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, _ := w.CreateFormFile("statement", file)
	part.Write(data)
	for k, v := range fields {
		w.WriteField(k, v)
	}
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/shazam/audit", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	SetupRouter().ServeHTTP(rec, req)

	var report models.AuditReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rec, report
}

func TestAuditOPayNigeriaInsights(t *testing.T) {
	full, report := postStatement(t, "opay_statement.csv", nil)
	if full.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", full.Code)
	}
	if report.Summary.AccountHolder != "CHINEDU OLUMIDE BALOGUN" {
		t.Errorf("account holder not extracted from header: %q", report.Summary.AccountHolder)
	}
	if report.Internal.SavingsDeposits == 0 || report.Internal.SelfTransfersOut == 0 {
		t.Errorf("OWealth savings and self-transfers should be detected: %+v", report.Internal)
	}
	for _, m := range report.CashLeaks.MidnightLeak.TopMerchants {
		if m.Merchant == "OPay OWealth Savings & Stash" {
			t.Errorf("savings must not appear as a midnight leak")
		}
	}
	if report.Insights.BettingSpend != 1000 || report.Insights.ReligiousGiving != 60000 {
		t.Errorf("unexpected insights: %+v", report.Insights)
	}

	summary, compact := postStatement(t, "opay_statement.csv", map[string]string{"detail": "summary"})
	if compact.Transactions != nil || summary.Body.Len()*3 > full.Body.Len() {
		t.Errorf("summary view should drop transactions (%d vs %d bytes)", summary.Body.Len(), full.Body.Len())
	}
	if compact.Summary.TotalOutflow != report.Summary.TotalOutflow {
		t.Errorf("summary view must keep KPIs")
	}
}
