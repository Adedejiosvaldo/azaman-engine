package api

import (
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

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
}

func logMetric(event string) {
	log.Printf("[AZAMAN-METRIC] event=%s", event)
}

// recordAudit counts a finished audit and how many statements it covered.
func recordAudit(statements int) {
	auditsCompleted.Add(1)
	statementsAudited.Add(int64(statements))
	if statements > 1 {
		multiAccountRuns.Add(1)
	}
	logMetric("audit_completed")
}

func recordAuditFailure() {
	auditsFailed.Add(1)
	logMetric("audit_failed")
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
	c.JSON(http.StatusOK, gin.H{
		"since":              metricsSince.UTC().Format(time.RFC3339),
		"audits_completed":   auditsCompleted.Load(),
		"audits_failed":      auditsFailed.Load(),
		"statements_audited": statementsAudited.Load(),
		"multi_account_runs": multiAccountRuns.Load(),
		"events":             events,
	})
}
