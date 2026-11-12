package api

import (
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"azaman-engine/internal/models"
	"github.com/gin-gonic/gin"
)

// Privacy-safe counters: numbers only, never statement content, names, amounts or IP addresses.
// They live in memory and reset on restart; every increment is also written to the log as one
// line ("[AZAMAN-METRIC] event=audit_completed"), so totals survive in the container logs.

var (
	auditsCompleted   atomic.Int64
	statementsAudited atomic.Int64
	multiAccountRuns  atomic.Int64
	auditsFailed      atomic.Int64
	metricsSince      = time.Now()
)

// allowedEvents are the client-side events the landing page may report. Anything else is rejected,
// so the endpoint cannot be used to store arbitrary data.
var allowedEvents = map[string]*atomic.Int64{
	"grade_shared":         new(atomic.Int64),
	"report_requested":     new(atomic.Int64),
	"full_picture_clicked": new(atomic.Int64),
	"tax_pack_clicked":     new(atomic.Int64),
	"whatsapp_clicked":     new(atomic.Int64),
	"purchase_started":     new(atomic.Int64),
	"grade_link_opened":    new(atomic.Int64), // someone opened a shared grade
	"audit_from_share":     new(atomic.Int64), // ...and then ran their own audit
	"bank_requested":       new(atomic.Int64), // asked for a bank we can't read yet
	// First or repeat check from this browser, judged by a date the browser keeps (nothing is
	// stored here, and the browser is not identified).
	"first_audit":       new(atomic.Int64),
	"repeat_audit_7d":   new(atomic.Int64), // last check was under 7 days ago
	"repeat_audit_30d":  new(atomic.Int64), // 7 to 30 days ago
	"repeat_audit_late": new(atomic.Int64), // over 30 days ago
}

// Breakdowns: per bank, per day, and the printed-totals check. Counts only.
type bankCount struct {
	Statements int            `json:"statements"`        // statements read successfully
	Failed     int            `json:"failed"`            // statements that could not be read
	Reasons    map[string]int `json:"reasons,omitempty"` // why: password, unreadable, no_transactions
	TotalsOK   int            `json:"totals_ok"`         // statements whose parsed totals matched the printed ones
	TotalsOff  int            `json:"totals_off"`        // ...and those that did not: a misread statement
}

type dayCount struct {
	Audits int `json:"audits"`
	Failed int `json:"failed"`
}

// statsDays is how many days of daily counts are kept in memory.
const statsDays = 30

var (
	breakdownMu sync.Mutex
	byBank      = map[string]*bankCount{}
	byDay       = map[string]*dayCount{}
)

func bankEntry(bank string) *bankCount {
	if bank == "" {
		bank = "Generic Bank"
	}
	b := byBank[bank]
	if b == nil {
		b = &bankCount{}
		byBank[bank] = b
	}
	return b
}

// dayEntry returns today's counts (Lagos time) and drops days older than statsDays.
func dayEntry() *dayCount {
	now := time.Now().In(lagos)
	key := now.Format("2006-01-02")
	d := byDay[key]
	if d == nil {
		d = &dayCount{}
		byDay[key] = d
		cutoff := now.AddDate(0, 0, -statsDays).Format("2006-01-02")
		for k := range byDay {
			if k < cutoff {
				delete(byDay, k)
			}
		}
	}
	return d
}

var lagos = func() *time.Location {
	if loc, err := time.LoadLocation("Africa/Lagos"); err == nil {
		return loc
	}
	return time.FixedZone("WAT", 3600)
}()

func logMetric(event string) {
	log.Printf("[AZAMAN-METRIC] event=%s", event)
}

// recordAudit counts a finished audit: each statement under its bank, and whether its parsed
// totals matched the totals printed on it.
func recordAudit(accounts []models.AccountSummary) {
	auditsCompleted.Add(1)
	statementsAudited.Add(int64(len(accounts)))
	if len(accounts) > 1 {
		multiAccountRuns.Add(1)
	}
	breakdownMu.Lock()
	dayEntry().Audits++
	for _, a := range accounts {
		b := bankEntry(a.Bank)
		b.Statements++
		if a.Check.Checked {
			if a.Check.OK {
				b.TotalsOK++
			} else {
				b.TotalsOff++
			}
		}
	}
	breakdownMu.Unlock()
	logMetric("audit_completed")
}

// recordAuditFailure counts a statement that could not be read, under its bank and the reason.
func recordAuditFailure(bank, reason string) {
	auditsFailed.Add(1)
	breakdownMu.Lock()
	dayEntry().Failed++
	b := bankEntry(bank)
	b.Failed++
	if b.Reasons == nil {
		b.Reasons = map[string]int{}
	}
	b.Reasons[reason]++
	breakdownMu.Unlock()
	log.Printf("[AZAMAN-METRIC] event=audit_failed bank=%q reason=%s", bank, reason)
}

// HandleEvent counts one named client event. Body: {"event": "grade_shared"}.
func HandleEvent(c *gin.Context) {
	var req struct {
		Event string `json:"event"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event is required"})
		return
	}
	counter, ok := allowedEvents[req.Event]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown event"})
		return
	}
	counter.Add(1)
	logMetric(req.Event)
	c.Status(http.StatusNoContent)
}

// HandleStats returns the counters. It is off unless STATS_TOKEN is set, and then needs
// "Authorization: Bearer <STATS_TOKEN>".
func HandleStats(c *gin.Context) {
	token := os.Getenv("STATS_TOKEN")
	if token == "" {
		c.Status(http.StatusNotFound)
		return
	}
	got := c.GetHeader("Authorization")
	want := "Bearer " + token
	if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		c.Status(http.StatusUnauthorized)
		return
	}
	events := make(gin.H, len(allowedEvents))
	for name, counter := range allowedEvents {
		events[name] = counter.Load()
	}
	breakdownMu.Lock()
	banks := make(map[string]bankCount, len(byBank))
	for name, b := range byBank {
		copied := *b
		copied.Reasons = make(map[string]int, len(b.Reasons))
		for k, v := range b.Reasons {
			copied.Reasons[k] = v
		}
		banks[name] = copied
	}
	type day struct {
		Date string `json:"date"`
		dayCount
	}
	days := make([]day, 0, len(byDay))
	for k, d := range byDay {
		days = append(days, day{k, *d})
	}
	breakdownMu.Unlock()
	sort.Slice(days, func(i, j int) bool { return days[i].Date > days[j].Date })

	c.JSON(http.StatusOK, gin.H{
		"since":              metricsSince.UTC().Format(time.RFC3339),
		"audits_completed":   auditsCompleted.Load(),
		"audits_failed":      auditsFailed.Load(),
		"statements_audited": statementsAudited.Load(),
		"multi_account_runs": multiAccountRuns.Load(),
		"events":             events,
		"by_bank":            banks,
		"by_day":             days, // newest first, last 30 days, Lagos time
	})
}
