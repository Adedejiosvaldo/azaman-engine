package api

import (
	"bytes"
	"io"
	"math"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"azaman-engine/internal/ai"
	"azaman-engine/internal/analyzer"
	"azaman-engine/internal/ephemeral"
	"azaman-engine/internal/models"
	"azaman-engine/internal/parser"
	"azaman-engine/internal/shazam"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var serverStartTime = time.Now()

// DecodeMemoRequest represents single memo decode payload.
type DecodeMemoRequest struct {
	Memo     string  `json:"memo" binding:"required"`
	Amount   float64 `json:"amount"`
	Date     string  `json:"date"`
	Currency string  `json:"currency"`
}

// HandleAudit processes uploaded bank statements ephemerally in RAM.
func HandleAudit(c *gin.Context) {
	startTime := time.Now()
	sessionID := uuid.New().String()

	// 1. Get uploaded file (either 'statement' or 'file' form key)
	fileHeader, err := c.FormFile("statement")
	if err != nil {
		fileHeader, err = c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Missing statement file. Provide a multipart file with key 'statement' or 'file'",
				"session": sessionID,
			})
			return
		}
	}

	// 2. Open file directly in memory
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read statement payload"})
		return
	}
	defer file.Close()

	// 3. Read entirely into RAM buffer
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to buffer statement in RAM"})
		return
	}

	// Ensure buffer is scrubbed when function exits
	defer func() {
		ephemeral.ScrubBytes(fileBytes)
		ephemeral.ForceEphemeralCleanup()
	}()

	fileSize := int64(len(fileBytes))
	fileExt := strings.ToLower(filepath.Ext(fileHeader.Filename))
	password := c.DefaultPostForm("password", "")
	bankOverride := c.DefaultPostForm("bank", "")
	accountHolder := strings.TrimSpace(c.DefaultPostForm("account_name", ""))

	// 4. Detect bank if not provided
	bankName := bankOverride
	if bankName == "" {
		bankName = parser.DetectBankFromFilename(fileHeader.Filename)
	}
	if bankName == "" {
		sampleSize := 4096
		if len(fileBytes) < sampleSize {
			sampleSize = len(fileBytes)
		}
		bankName = parser.DetectBank(string(fileBytes[:sampleSize]))
	}
	if accountHolder == "" && fileExt != ".pdf" {
		sampleSize := 4096
		if len(fileBytes) < sampleSize {
			sampleSize = len(fileBytes)
		}
		accountHolder = parser.ExtractAccountHolder(string(fileBytes[:sampleSize]))
	}

	var rawTransactions []models.RawTransaction

	// 5. Route to appropriate ephemeral parser
	switch fileExt {
	case ".pdf":
		var detectedBank string
		rawTransactions, detectedBank, err = parser.ParsePDF(fileBytes, password, bankName)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error":   "PDF statement extraction failed: " + err.Error(),
				"session": sessionID,
			})
			return
		}
		if bankOverride == "" && detectedBank != "" {
			bankName = detectedBank
		}

	case ".xlsx", ".xls":
		rawTransactions, err = parser.ParseXLSX(fileBytes, bankName)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error":   "Excel statement extraction failed: " + err.Error(),
				"session": sessionID,
			})
			return
		}

	default: // .csv, .txt, .tsv or generic stream
		rawTransactions, err = parser.ParseCSV(bytes.NewReader(fileBytes), bankName)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error":   "CSV statement extraction failed: " + err.Error(),
				"session": sessionID,
			})
			return
		}
	}

	if len(rawTransactions) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "No transactions could be parsed from the provided statement.",
			"session": sessionID,
			"bank":    bankName,
		})
		return
	}

	// 6. Shazam Enricher: Clean noise, match merchants, tag flags
	decodedTxs := make([]models.DecodedTransaction, len(rawTransactions))
	for i, raw := range rawTransactions {
		decodedTxs[i] = shazam.MatchAndDecode(raw)
	}

	// 6a. User labels: people the user marked as vendors or family in the dashboard (kept in their browser only)
	analyzer.ApplyContactLabels(decodedTxs, c.PostFormArray("vendor"), c.PostFormArray("family"), shazam.VendorCategory)

	// 6b. Optional Laya AI System 1 Decision Pass for Ambiguous/Low-Confidence Memos
	layaClient := ai.GetLayaClient()
	if layaClient.IsAvailable() {
		var ambiguousItems []ai.LayaMemoItem
		seenMemos := make(map[string]bool)
		for _, tx := range decodedTxs {
			cleanKey := strings.TrimSpace(strings.ToLower(tx.CleanMemo))
			if cleanKey == "" {
				cleanKey = strings.TrimSpace(strings.ToLower(tx.RawDescription))
			}
			if tx.Confidence < 0.90 && tx.Amount < 0 && layaEligible(tx) && !seenMemos[cleanKey] {
				seenMemos[cleanKey] = true
				ambiguousItems = append(ambiguousItems, ai.LayaMemoItem{
					ID:       tx.ID,
					Memo:     tx.CleanMemo,
					Amount:   math.Abs(tx.Amount),
					RawMemo:  tx.RawDescription,
					Hour:     knownHour(tx),
					IsCredit: tx.Credit > 0,
					Currency: tx.Currency,
				})
				// Cap batch to 5 items to guarantee sub-second responses
				if len(ambiguousItems) >= 5 {
					break
				}
			}
		}

		if len(ambiguousItems) > 0 {
			if decisions, err := layaClient.ClassifyBatch(ambiguousItems); err == nil {
				memoDecisions := make(map[string]ai.LayaDecision)
				for _, item := range ambiguousItems {
					if dec, ok := decisions[item.ID]; ok {
						key := strings.TrimSpace(strings.ToLower(item.Memo))
						if key == "" {
							key = strings.TrimSpace(strings.ToLower(item.RawMemo))
						}
						memoDecisions[key] = dec
					}
				}

				for i := range decodedTxs {
					tx := &decodedTxs[i]
					key := strings.TrimSpace(strings.ToLower(tx.CleanMemo))
					if key == "" {
						key = strings.TrimSpace(strings.ToLower(tx.RawDescription))
					}
					dec, ok := decisions[tx.ID]
					if !ok {
						dec, ok = memoDecisions[key]
					}
					// Only override when Laya is more confident than the rules, and never for rows
					// whose type is already certain (fees, savings, loans).
					if ok && layaEligible(*tx) && dec.Confidence > tx.Confidence {
						if dec.Category != "" {
							tx.Category = dec.Category
						}
						if dec.Classification != "" {
							tx.Classification = dec.Classification
						}
						if dec.Merchant != "" && tx.Confidence < 0.85 {
							tx.Merchant = dec.Merchant
						}
						if dec.Confidence > tx.Confidence {
							tx.Confidence = dec.Confidence
						}
						if dec.IsSilentLeak {
							tx.Flags.IsMidnightLeak = true
						}
						if dec.IsUrgent2k {
							tx.Flags.IsUrgent2k = true
						}
						if dec.IsTaxDeductible {
							tx.Flags.IsTaxDeductible = true
						}
					}
				}
			}
		}
	}

	// Scrub raw transactions slice
	ephemeral.ScrubRawTransactions(rawTransactions)

	// 7. Leak Analysis & Runway Calculation
	formatName := strings.TrimPrefix(fileExt, ".")
	if formatName == "" {
		formatName = "csv"
	}

	audit := analyzer.GenerateAudit(
		decodedTxs,
		bankName,
		sessionID,
		formatName,
		fileSize,
		startTime,
		accountHolder,
	)

	// detail=summary drops per-transaction arrays (~95% smaller) for low-end phones and costly data.
	if c.DefaultPostForm("detail", c.Query("detail")) == "summary" {
		compactReport(&audit)
	}

	c.Header("X-Session-ID", sessionID)
	c.Header("X-Response-Time", time.Since(startTime).String())
	c.JSON(http.StatusOK, audit)
}

// HandleDecodeMemo decodes an individual transaction memo in sub-5ms.
func HandleDecodeMemo(c *gin.Context) {
	var req DecodeMemoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request. 'memo' string is required"})
		return
	}

	parsedDate, _ := parser.ParseDate(req.Date)

	raw := models.RawTransaction{
		Description: req.Memo,
		Debit:       req.Amount,
		ParsedDate:  parsedDate,
		Currency:    req.Currency,
	}

	decoded := shazam.MatchAndDecode(raw)
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"decoded": decoded,
	})
}

// HandleGetMerchants returns registered merchant patterns and classification taxonomy.
func HandleGetMerchants(c *gin.Context) {
	type merchantInfo struct {
		Merchant       string   `json:"merchant"`
		Category       string   `json:"category"`
		Classification string   `json:"classification"`
		Patterns       []string `json:"patterns"`
		IsLateNight    bool     `json:"is_late_night"`
		IsTechVendor   bool     `json:"is_tech_vendor"`
	}

	list := make([]merchantInfo, len(shazam.MerchantRegistry))
	for i, rule := range shazam.MerchantRegistry {
		list[i] = merchantInfo{
			Merchant:       rule.Merchant,
			Category:       rule.Category,
			Classification: rule.Classification,
			Patterns:       rule.Patterns,
			IsLateNight:    rule.IsLateNight,
			IsTechVendor:   rule.IsTechVendor,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_merchants": len(list),
		"merchants":       list,
	})
}

// HandleHealth provides engine status, uptime, and ephemeral RAM stats.
func HandleHealth(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	bToMb := func(b uint64) float64 {
		return float64(b) / 1024 / 1024
	}

	layaAvailable := ai.GetLayaClient().IsAvailable()
	layaStatus := "offline_fallback"
	if layaAvailable {
		layaStatus = "connected"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "azaman-shazam-engine",
		"uptime":  time.Since(serverStartTime).String(),
		"version": "1.0.0",
		"ai_decision_engine": gin.H{
			"provider": "Laya AI System 1 Decision Model",
			"status":   layaStatus,
			"endpoint": ai.GetLayaClient().BaseURL,
		},
		"memory_mb": gin.H{
			"alloc":       bToMb(m.Alloc),
			"total_alloc": bToMb(m.TotalAlloc),
			"sys":         bToMb(m.Sys),
			"num_gc":      m.NumGC,
		},
		"disk_free_compliance": "Ephemeral RAM Only - 0 bytes persisted",
	})
}

// compactReport strips per-transaction arrays, keeping every KPI, leak total, and insight.
func compactReport(r *models.AuditReport) {
	r.Transactions = nil
	r.CashLeaks.MidnightLeak.Transactions = nil
	r.CashLeaks.Urgent2k.Transactions = nil
	r.CashLeaks.BankCharges.Transactions = nil
	r.Reversals.UnresolvedDebits = nil
	for i := range r.CashLeaks.DuplicateDebits.Pairs {
		r.CashLeaks.DuplicateDebits.Pairs[i].Transactions = nil
	}
}

// layaEligible reports whether a transaction may be sent to, or reclassified by, the Laya sidecar.
func layaEligible(tx models.DecodedTransaction) bool {
	return !tx.Flags.IsBankFee && !tx.Flags.IsSavings && !tx.Flags.IsLoan
}

// knownHour returns the transaction's hour of day, or nil for date-only rows (parsed as 00:00:00).
func knownHour(tx models.DecodedTransaction) *int {
	t := tx.ParsedDate
	if t.IsZero() || (t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0) {
		return nil
	}
	h := t.Hour()
	return &h
}
