package ai

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSlowSidecarIsPausedAfterTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		time.Sleep(200 * time.Millisecond) // slower than the client allows
	}))
	defer srv.Close()

	c := NewLayaClient(srv.URL, 50*time.Millisecond)
	if !c.IsAvailable() {
		t.Fatal("sidecar should start available")
	}
	if _, err := c.ClassifyBatch([]LayaMemoItem{{ID: "1", Memo: "golden crust"}}); err == nil {
		t.Fatal("expected a timeout")
	}
	if c.IsAvailable() {
		t.Error("after a timeout the engine should stop asking for a while")
	}
}
