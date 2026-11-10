package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"azaman-engine/internal/models"
)

func proofReport(format string) *models.AuditReport {
	r := &models.AuditReport{}
	r.Grade = models.FinancialGrade{Grade: "B+", Score: 78, Focus: "drains", FocusAmount: 12000}
	r.SpendingMix.Buckets = []models.SpendingBucket{
		{Key: "builders", Amount: 50000, Percentage: 20},
		{Key: "drains", Amount: 30000, Percentage: 12.5},
	}
	r.Monthly.Months = []models.MonthlyFlow{{Month: "2026-01", Grade: "C"}, {Month: "2026-02", Grade: "B+", Partial: true}}
	r.Monthly.PositiveStreak = 2
	r.Summary.Currency = "NGN"
	r.Summary.NetRetained = 84000.5
	r.AuditMeta.FileFormat = format
	return r
}

func TestGradeProofSignsShareNumbers(t *testing.T) {
	t.Setenv("GRADE_PROOF_SECRET", "test-secret")
	now := time.Unix(1_800_000_000, 0)
	p := gradeProof(proofReport("pdf"), now)
	if p == nil {
		t.Fatal("no proof for a PDF audit")
	}

	mac := hmac.New(sha256.New, []byte("test-secret"))
	mac.Write([]byte(p.Payload))
	if want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil)); p.Sig != want {
		t.Fatalf("sig %q, want %q", p.Sig, want)
	}

	var got gradeProofPayload
	if err := json.Unmarshal([]byte(p.Payload), &got); err != nil {
		t.Fatal(err)
	}
	if got.Grade.Grade != "B+" || got.Monthly.PositiveStreak != 2 || got.Summary.NetRetained != 84000.5 ||
		got.AuditMeta.FileFormat != "pdf" || got.IssuedAt != now.Unix() {
		t.Fatalf("payload lost share numbers: %+v", got)
	}
	if len(got.SpendingMix.Buckets) != 1 || got.SpendingMix.Buckets[0].Key != "drains" {
		t.Fatalf("payload should carry only the drains bucket: %+v", got.SpendingMix.Buckets)
	}
	if len(got.Monthly.Months) != 2 || !got.Monthly.Months[1].Partial {
		t.Fatalf("months not carried: %+v", got.Monthly.Months)
	}
}

func TestGradeProofOnlyForPDFWithSecret(t *testing.T) {
	t.Setenv("GRADE_PROOF_SECRET", "")
	if gradeProof(proofReport("pdf"), time.Now()) != nil {
		t.Error("proof issued without a secret")
	}
	t.Setenv("GRADE_PROOF_SECRET", "test-secret")
	if gradeProof(proofReport("csv"), time.Now()) != nil {
		t.Error("a CSV grade proves nothing and must not get a proof")
	}
	r := proofReport("pdf")
	r.Grade.Grade = ""
	if gradeProof(r, time.Now()) != nil {
		t.Error("proof issued with no grade")
	}
}
