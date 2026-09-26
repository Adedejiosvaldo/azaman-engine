package ai

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClassifyBatchRejectsErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"detail":[{"msg":"too many items"}]}`))
	}))
	defer srv.Close()

	c := &LayaClient{BaseURL: srv.URL, httpClient: srv.Client()}
	if _, err := c.ClassifyBatch([]LayaMemoItem{{ID: "1", Memo: "x", Amount: 1}}); err == nil {
		t.Fatal("expected error for 422 response, got nil")
	}
}
