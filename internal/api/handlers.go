package api

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"mime/multipart"
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

// maxStatements caps how many statements one audit accepts. The 32MB body cap still applies to all of them together.
const maxStatements = 6

// statementError is a per-file failure.
type statementError struct {
	status int
	msg    string
}

// parsedStatement is one uploaded file, parsed and decoded.
type parsedStatement struct {
	account models.AccountSummary
	txs     []models.DecodedTransaction
	size    int64
}

// HandleAudit processes one or more uploaded bank statements ephemerally in RAM. Several statements
// (form key "statements", repeated) are audited together as one picture: transfers between the
// holder's own accounts are matched across files and every row is tagged with its account.
func HandleAudit(c *gin.Context) {
	startTime := time.Now()
	sessionID := uuid.New().String()

	// 1. Collect uploaded files: repeated 'statements', or a single 'statement' / 'file'
	var headers []*multipart.FileHeader
	if form, err := c.MultipartForm(); err == nil && form != nil {
		for _, key := range []string{"statements", "statement", "file"} {
			if len(headers) == 0 {
				headers = append(headers, form.File[key]...)
			}
		}
	}
	if len(headers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Missing statement file. Provide a multipart file with key 'statement', 'file' or 'statements'",
			"session": sessionID,
		})
		return
	}
	if len(headers) > maxStatements {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   fmt.Sprintf("Too many statements: send at most %d at once.", maxStatements),
			"session": sessionID,
		})
		return
	}

	// Per-file options line up with the files by position; single values apply to every file.
	passwords := c.PostFormArray("passwords")
	banks := c.PostFormArray("banks")
	sharedPassword := c.DefaultPostForm("password", "")
	sharedBank := c.DefaultPostForm("bank", "")
	accountHolder := strings.TrimSpace(c.DefaultPostForm("account_name", ""))
	pick := func(list []string, i int, fallback string) string {
		if i < len(list) && list[i] != "" {
			return list[i]
		}
		return fallback
	}

	var statements []parsedStatement
	labelCount := make(map[string]int)
	for i, fh := range headers {
		st, serr := parseStatement(fh, pick(passwords, i, sharedPassword), pick(banks, i, sharedBank), accountHolder)
		if serr != nil {
			body := gin.H{"error": serr.msg, "session": sessionID}
			if len(headers) > 1 {
				body["file"] = fh.Filename
				body["file_index"] = i
			}
			recordAuditFailure()
			c.JSON(serr.status, body)
			return
		}
		// Label each account by bank; a second statement from the same bank becomes "GTBank 2".
		label := st.account.Bank
		if label == "" {
			label = strings.TrimSuffix(fh.Filename, filepath.Ext(fh.Filename))
		}
		labelCount[label]++
		if n := labelCount[label]; n > 1 {
			label = fmt.Sprintf("%s %d", label, n)
		}
		st.account.Label = label
		for j := range st.txs {
			st.txs[j].Account = label
		}
		if accountHolder == "" {
			accountHolder = st.account.AccountHolder
		}
		statements = append(statements, st)
	}

	var decodedTxs []models.DecodedTransaction
	var accounts []models.AccountSummary
	var fileSize int64
	formats := make(map[string]bool)
	for _, st := range statements {
		decodedTxs = append(decodedTxs, st.txs...)
		accounts = append(accounts, st.account)
		fileSize += st.size
		formats[st.account.FileFormat] = true
	}

	bankName := accounts[0].Bank
	formatName := accounts[0].FileFormat
	check := accounts[0].Check
	if len(accounts) > 1 {
		labels := make([]string, len(accounts))
		for i, a := range accounts {
			labels[i] = a.Label
		}
		bankName = strings.Join(labels, " + ")
		if len(formats) > 1 {
			formatName = "mixed"
		}
		// Surface the first statement whose totals did not add up, if any.
		for _, a := range accounts {
			if a.Check.Checked && !a.Check.OK {
				check = a.Check
				break
			}
		}
	}

	// 6a. User labels: people the user marked as vendors or family in the dashboard (kept in their browser only)
	analyzer.ApplyContactLabels(decodedTxs, c.PostFormArray("vendor"), c.PostFormArray("family"), shazam.VendorCategory)
	analyzer.ApplyPaymentLabels(decodedTxs, c.PostFormArray("payment_label"), shazam.VendorCategory)

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
					Memo:     layaText(tx),
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
						if tx.Counterparty != "" {
							tx.DecisionBasis = "ai" // person scoring leaves AI-decided rows alone
						}
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
						// Laya's "silent leak" is about discretionary spend, not time of day: only
						// count it as a midnight leak when the row's time is known to be late.
						if h := knownHour(*tx); dec.IsSilentLeak && h != nil && (*h >= 22 || *h <= 3) {
							tx.Flags.IsMidnightLeak = true
						}
						if dec.IsUrgent2k {
							tx.Flags.IsUrgent2k = true
						}
						if dec.IsTaxDeductible {
							tx.Flags.IsTaxDeductible = true
						}
						// "Bought, not given": a payment to a person for goods or a service
						if dec.IsPurchase && tx.Counterparty != "" && tx.Debit > 0 {
							tx.Category = shazam.VendorCategory
							tx.Classification = "Personal Purchase"
							tx.Flags.IsUrgent2k = false
						}
					}
				}
			}
		}
	}

	// 7. Leak Analysis & Runway Calculation
	audit := analyzer.GenerateAudit(
		decodedTxs,
		bankName,
		sessionID,
		formatName,
		fileSize,
		startTime,
		accountHolder,
	)
	audit.Check = check
	analyzer.FillAccountTotals(accounts, audit.Transactions)
	audit.Accounts = accounts

	// detail=summary drops per-transaction arrays (~95% smaller) for low-end phones and costly data.
	if c.DefaultPostForm("detail", c.Query("detail")) == "summary" {
		compactReport(&audit)
	}

	recordAudit(len(statements))
	c.Header("X-Session-ID", sessionID)
	c.Header("X-Response-Time", time.Since(startTime).String())
	c.JSON(http.StatusOK, audit)
}

// parseStatement reads one uploaded file into RAM, detects its bank and holder, parses and decodes it.
// The file buffer and the raw rows are scrubbed before it returns.
func parseStatement(fh *multipart.FileHeader, password, bankOverride, accountHolder string) (parsedStatement, *statementError) {
	fail := func(status int, msg string) (parsedStatement, *statementError) {
		return parsedStatement{}, &statementError{status: status, msg: msg}
	}

	file, err := fh.Open()
	if err != nil {
		return fail(http.StatusInternalServerError, "Failed to read statement payload")
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return fail(http.StatusInternalServerError, "Failed to buffer statement in RAM")
	}
	defer func() {
		ephemeral.ScrubBytes(fileBytes)
		ephemeral.ForceEphemeralCleanup()
	}()

	fileExt := strings.ToLower(filepath.Ext(fh.Filename))
	sample := fileBytes
	if len(sample) > 4096 {
		sample = sample[:4096]
	}

	bankName := bankOverride
	if bankName == "" {
		bankName = parser.DetectBankFromFilename(fh.Filename)
	}
	if bankName == "" {
		bankName = parser.DetectBank(string(sample))
	}
	if accountHolder == "" {
		accountHolder = parser.ExtractHolderFromFilename(fh.Filename)
	}
	if accountHolder == "" && fileExt != ".pdf" {
		accountHolder = parser.ExtractAccountHolder(string(sample))
	}

	var rawTransactions []models.RawTransaction
	var check models.StatementCheck

	switch fileExt {
	case ".pdf":
		var detectedBank string
		rawTransactions, detectedBank, check, err = parser.ParsePDF(fileBytes, password, bankName)
		if err != nil {
			return fail(http.StatusUnprocessableEntity, "PDF statement extraction failed: "+err.Error())
		}
		if bankOverride == "" && detectedBank != "" {
			bankName = detectedBank
		}
	case ".xlsx", ".xls":
		rawTransactions, err = parser.ParseXLSX(fileBytes, bankName)
		if err != nil {
			return fail(http.StatusUnprocessableEntity, "Excel statement extraction failed: "+err.Error())
		}
	default: // .csv, .txt, .tsv or generic stream
		rawTransactions, err = parser.ParseCSV(bytes.NewReader(fileBytes), bankName)
		if err != nil {
			return fail(http.StatusUnprocessableEntity, "CSV statement extraction failed: "+err.Error())
		}
	}
	defer ephemeral.ScrubRawTransactions(rawTransactions)

	if len(rawTransactions) == 0 {
		return fail(http.StatusUnprocessableEntity, "No transactions could be parsed from the provided statement.")
	}

	decoded := make([]models.DecodedTransaction, len(rawTransactions))
	for i, raw := range rawTransactions {
		decoded[i] = shazam.MatchAndDecode(raw)
	}

	formatName := strings.TrimPrefix(fileExt, ".")
	if formatName == "" {
		formatName = "csv"
	}
	return parsedStatement{
		account: models.AccountSummary{
			Bank:          bankName,
			AccountHolder: accountHolder,
			FileFormat:    formatName,
			Transactions:  len(decoded),
			Check:         check,
		},
		txs:  decoded,
		size: int64(len(fileBytes)),
	}, nil
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
		"service": "azaman-engine",
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
// A transfer to a person with no typed purpose is left to person scoring: Laya reads memo text,
// and a bare name gives it nothing to go on (it guesses bills, loans or even betting).
func layaEligible(tx models.DecodedTransaction) bool {
	return !tx.Flags.IsBankFee && !tx.Flags.IsSavings && !tx.Flags.IsLoan && !tx.Flags.IsSelfTransfer &&
		(tx.Counterparty == "" || tx.Purpose != "")
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

// layaText is what Laya reads: the typed purpose of a person-to-person transfer when there is one
// (it carries the meaning; the name only adds noise), otherwise the cleaned memo.
func layaText(tx models.DecodedTransaction) string {
	if tx.Purpose != "" {
		return tx.Purpose
	}
	return tx.CleanMemo
}
