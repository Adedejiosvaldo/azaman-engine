package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"time"

	"azaman-engine/internal/models"
)

// The browser uploads statements straight to the engine (a hosting proxy would cap the upload),
// so the landing site no longer sees the report on its own server. To keep share links honest it
// needs proof that a grade came from the engine: a small signed copy of exactly the numbers the
// share card uses. Set GRADE_PROOF_SECRET to the same long random value on the engine and the
// landing site. Without it no proof is issued and share links still work, unverified.

type proofBucket struct {
	Key        string  `json:"key"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
}

type proofMonth struct {
	Partial bool   `json:"partial"`
	Grade   string `json:"grade,omitempty"`
}

// gradeProofPayload mirrors the parts of AuditReport that the landing site's buildShareFields reads.
type gradeProofPayload struct {
	Grade       models.FinancialGrade `json:"grade"`
	SpendingMix struct {
		Buckets []proofBucket `json:"buckets"`
	} `json:"spending_mix"`
	Monthly struct {
		Months         []proofMonth `json:"months"`
		PositiveStreak int          `json:"positive_streak"`
	} `json:"monthly"`
	Summary struct {
		Currency    string  `json:"currency"`
		NetRetained float64 `json:"net_retained"`
	} `json:"summary"`
	AuditMeta struct {
		FileFormat string `json:"file_format"`
	} `json:"audit_meta"`
	IssuedAt int64 `json:"issued_at"` // unix seconds; the landing site refuses old proofs
}

// gradeProof signs the share-card numbers of a PDF audit. A CSV or spreadsheet is easy to type up,
// so its grade proves nothing and gets no proof. Returns nil when there is nothing to sign.
func gradeProof(r *models.AuditReport, now time.Time) *models.GradeProof {
	secret := os.Getenv("GRADE_PROOF_SECRET")
	if secret == "" || r.Grade.Grade == "" || r.AuditMeta.FileFormat != "pdf" {
		return nil
	}
	var p gradeProofPayload
	p.Grade = r.Grade
	for _, b := range r.SpendingMix.Buckets {
		if b.Key == "drains" {
			p.SpendingMix.Buckets = append(p.SpendingMix.Buckets, proofBucket{Key: b.Key, Amount: b.Amount, Percentage: b.Percentage})
		}
	}
	for _, m := range r.Monthly.Months {
		p.Monthly.Months = append(p.Monthly.Months, proofMonth{Partial: m.Partial, Grade: m.Grade})
	}
	p.Monthly.PositiveStreak = r.Monthly.PositiveStreak
	p.Summary.Currency = r.Summary.Currency
	p.Summary.NetRetained = r.Summary.NetRetained
	p.AuditMeta.FileFormat = r.AuditMeta.FileFormat
	p.IssuedAt = now.Unix()

	raw, err := json.Marshal(p)
	if err != nil {
		return nil
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(raw)
	return &models.GradeProof{
		Payload: string(raw),
		Sig:     base64.RawURLEncoding.EncodeToString(mac.Sum(nil)),
	}
}
