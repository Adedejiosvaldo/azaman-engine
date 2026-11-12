package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postFile(t *testing.T, name string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, _ := w.CreateFormFile("statement", name)
	part.Write(content)
	w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shazam/audit", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	SetupRouter().ServeHTTP(rec, req)
	return rec
}

func getStats(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	SetupRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats: %d", rec.Code)
	}
	var out map[string]json.RawMessage
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

// Stats count per bank and per day, and say why a statement could not be read.
func TestStatsBreakdowns(t *testing.T) {
	t.Setenv("STATS_TOKEN", "test-token")
	var before map[string]bankCount
	json.Unmarshal(getStats(t)["by_bank"], &before)

	if rec, _ := postStatement(t, "kuda_statement.csv", nil); rec.Code != http.StatusOK {
		t.Fatalf("good statement: %d", rec.Code)
	}
	bad := postFile(t, "mystery.csv", []byte("hello\nthis is not a statement\n"))
	if bad.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unreadable statement: got %d", bad.Code)
	}
	// The failure says which bank and why, so the page can offer "request this bank".
	var failure struct{ Bank, Format, Reason string }
	json.Unmarshal(bad.Body.Bytes(), &failure)
	if failure.Format != "csv" || failure.Reason != "no_transactions" || failure.Bank == "" {
		t.Errorf("failure body: %+v", failure)
	}

	stats := getStats(t)
	var after map[string]bankCount
	json.Unmarshal(stats["by_bank"], &after)
	if after["Kuda Bank"].Statements != before["Kuda Bank"].Statements+1 {
		t.Errorf("Kuda statement not counted: before %+v after %+v", before["Kuda Bank"], after["Kuda Bank"])
	}
	if after[failure.Bank].Reasons["no_transactions"] != before[failure.Bank].Reasons["no_transactions"]+1 {
		t.Errorf("failure reason not counted: %+v", after[failure.Bank])
	}

	var days []struct {
		Date   string
		Audits int
		Failed int
	}
	json.Unmarshal(stats["by_day"], &days)
	if len(days) == 0 || days[0].Audits == 0 || days[0].Failed == 0 {
		t.Errorf("today's counts missing: %+v", days)
	}

	// Nothing personal leaks into the stats: no file names, no holder, no amounts.
	raw, _ := json.Marshal(stats)
	for _, private := range []string{"mystery", "TUNDE", "BAKARE"} {
		if strings.Contains(string(raw), private) {
			t.Errorf("stats contain %q", private)
		}
	}
}

func TestStatsCountReturningVisitors(t *testing.T) {
	t.Setenv("STATS_TOKEN", "test-token")
	for _, ev := range []string{"first_audit", "repeat_audit_7d", "bank_requested"} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(`{"event":"`+ev+`"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		SetupRouter().ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("event %s rejected: %d", ev, rec.Code)
		}
	}
	var events map[string]int
	json.Unmarshal(getStats(t)["events"], &events)
	if events["first_audit"] == 0 || events["repeat_audit_7d"] == 0 || events["bank_requested"] == 0 {
		t.Errorf("events not counted: %+v", events)
	}
}
