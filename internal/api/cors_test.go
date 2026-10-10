package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"azaman-engine/internal/models"
)

func TestCORSNoCredentialsWithoutAllowlist(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "")
	r := SetupRouter()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Header().Get("Access-Control-Allow-Credentials") == "true" {
		t.Errorf("credentials must not be allowed for arbitrary origins")
	}
}

func TestLayaSkipsBareNameTransfers(t *testing.T) {
	bare := models.DecodedTransaction{Counterparty: "CYRIL SUNDAY AYI"}
	typed := models.DecodedTransaction{Counterparty: "CYRIL SUNDAY AYI", Purpose: "curtains"}
	if layaEligible(bare) {
		t.Error("a transfer with only a name should not go to Laya")
	}
	if !layaEligible(typed) {
		t.Error("a transfer with a typed purpose should go to Laya")
	}
}
