package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
