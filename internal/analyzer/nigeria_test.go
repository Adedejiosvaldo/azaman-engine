package analyzer

import (
	"testing"
	"time"

	"azaman-engine/internal/models"
)

func debit(desc string, amt float64, at time.Time) models.DecodedTransaction {
	return models.DecodedTransaction{RawDescription: desc, CleanMemo: desc, Merchant: desc, Amount: -amt, Debit: amt, ParsedDate: at, Date: at.Format(time.RFC3339), Currency: "NGN"}
}

func credit(desc string, amt float64, at time.Time) models.DecodedTransaction {
	return models.DecodedTransaction{RawDescription: desc, CleanMemo: desc, Merchant: desc, Amount: amt, Credit: amt, ParsedDate: at, Date: at.Format(time.RFC3339), Currency: "NGN"}
}

var t0 = time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)

func TestDetectReversals(t *testing.T) {
	txs := []models.DecodedTransaction{
		debit("Transfer to JOHN DOE | GTBank", 20000, t0),
		credit("REVERSAL: Transfer to JOHN DOE", 20000, t0.Add(72*time.Hour)),
		debit("NIP transfer FAILED - pending", 7000, t0),
	}
	txs[0].Flags.IsUrgent2k = true

	r := DetectReversals(txs)
	if r.ReversedCount != 1 || r.ReversedAmount != 20000 {
		t.Fatalf("expected 1 reversal of 20000, got %d / %.2f", r.ReversedCount, r.ReversedAmount)
	}
	if !r.Pairs[0].Late || r.LateReversals != 1 {
		t.Errorf("72h reversal should be flagged late (CBN 48h)")
	}
	if !txs[0].Flags.IsReversal || !txs[1].Flags.IsReversal || txs[0].Flags.IsUrgent2k {
		t.Errorf("both legs must be flagged as reversal and leave Urgent 2k")
	}
	if txs[0].IsSpend() || txs[1].IsEarning() {
		t.Errorf("reversed legs must not count as spend or income")
	}
	if r.UnresolvedCount != 1 || r.UnresolvedAmount != 7000 {
		t.Errorf("expected failed unreversed debit as dispute candidate, got %d", r.UnresolvedCount)
	}
}

func TestDetectSelfTransfers(t *testing.T) {
	txs := []models.DecodedTransaction{
		debit("Transfer to BALOGUN, OLUMIDE CHINEDU | Kuda MFB", 1050, t0), // holder, reordered
		debit("Transfer to FAVOUR IFEOLUWA BALOGUN | OPay", 5000, t0),      // sibling: shared surname only
		credit("Add Money-Bank Card | Guaranty Trust Bank", 2000, t0),      // own card top-up
		{RawDescription: "Auto-save to OWealth", Amount: -8390, Debit: 8390, Flags: models.TransactionFlags{IsSavings: true, IsMidnightLeak: true}},
	}
	r := DetectSelfTransfers(txs, "CHINEDU OLUMIDE BALOGUN")

	if !txs[0].Flags.IsSelfTransfer {
		t.Errorf("transfer to holder's own name should be a self-transfer")
	}
	if txs[1].Flags.IsSelfTransfer {
		t.Errorf("sibling sharing only a surname must not be a self-transfer")
	}
	if !txs[2].Flags.IsSelfTransfer {
		t.Errorf("Add Money from own card should be a self-transfer")
	}
	if txs[3].Flags.IsMidnightLeak {
		t.Errorf("savings deposit must not be a midnight leak")
	}
	if r.SelfTransfersOut != 1050 || r.SelfTransfersIn != 2000 || r.SavingsDeposits != 8390 {
		t.Errorf("unexpected internal report: %+v", r)
	}
}

func TestDetectRecurringRemovesUrgent2k(t *testing.T) {
	var txs []models.DecodedTransaction
	for w := 0; w < 4; w++ {
		tx := debit("Transfer to MUM ADEYEMI - upkeep", 5000, t0.AddDate(0, 0, 7*w))
		tx.Category = "Family & Favors"
		tx.Flags.IsUrgent2k = true
		txs = append(txs, tx)
	}
	// Irregular favors to one friend stay as Urgent 2k
	for _, d := range []int{0, 1, 9} {
		tx := debit("Transfer to SEGUN BELLO", float64(2000+d*300), t0.AddDate(0, 0, d))
		tx.Flags.IsUrgent2k = true
		txs = append(txs, tx)
	}

	rec := DetectRecurring(txs)
	if len(rec) != 1 {
		t.Fatalf("expected 1 recurring series, got %d: %+v", len(rec), rec)
	}
	if rec[0].Kind != "Black Tax (Family Support)" || rec[0].Count != 4 || rec[0].IntervalDays != 7 {
		t.Errorf("unexpected series: %+v", rec[0])
	}
	for i := 0; i < 4; i++ {
		if txs[i].Flags.IsUrgent2k || !txs[i].Flags.IsRecurring {
			t.Errorf("weekly upkeep should be recurring, not Urgent 2k")
		}
	}
	if !txs[4].Flags.IsUrgent2k {
		t.Errorf("irregular favors should stay Urgent 2k")
	}
}

func TestBankChargesOvercharges(t *testing.T) {
	fee := func(desc string, amt float64, at time.Time) models.DecodedTransaction {
		tx := debit(desc, amt, at)
		tx.Flags.IsBankFee = true
		tx.Category = "Bank Fees & Charges"
		return tx
	}
	txs := []models.DecodedTransaction{
		credit("Transfer from CLIENT", 50000, t0),
		fee("Electronic Money Transfer Levy", 50, t0),
		fee("Electronic Money Transfer Levy", 50, t0.Add(time.Hour)), // no second ₦10k receipt
		fee("NIP CHG transfer fee", 100, t0),                         // above ₦53.75 cap
		fee("VAT on NIP", 3.75, t0),
		fee("Card maintenance fee", 50, t0),
		fee("Card maintenance fee", 50, t0.AddDate(0, 1, 0)), // ₦100 in one quarter
		fee("Cybersecurity levy", 25, t0),
	}
	r := AnalyzeBankCharges(txs)

	if r.EMTL != 100 || r.VATOnFees != 3.75 || r.CyberLevy != 25 || r.TransferFees != 100 {
		t.Errorf("fee breakdown wrong: %+v", r)
	}
	if len(r.Overcharges) != 3 {
		t.Fatalf("expected 3 overcharges (EMTL, NIP, card), got %d: %+v", len(r.Overcharges), r.Overcharges)
	}
	if r.TotalOvercharge != 50+46.25+50 {
		t.Errorf("expected total overcharge 146.25, got %.2f", r.TotalOvercharge)
	}
}

func TestEstimateTaxNigeriaTaxAct(t *testing.T) {
	var txs []models.DecodedTransaction
	for m := 0; m < 12; m++ {
		txs = append(txs, credit("Salary", 1_000_000, t0.AddDate(0, m, 0)))
	}
	loan := credit("FairMoney loan disbursement", 500_000, t0)
	loan.Flags.IsLoan = true
	txs = append(txs, loan)

	est := EstimateTax(txs, 365, "NGN")
	if est.AnnualizedIncome != 12_000_000 {
		t.Fatalf("loans must not count as income; got %.2f", est.AnnualizedIncome)
	}
	// 0% on 800k + 15% on 2.2M + 18% on 9M = 330,000 + 1,620,000
	if est.EstimatedAnnualTax != 1_950_000 {
		t.Errorf("expected ₦1,950,000 tax, got %.2f", est.EstimatedAnnualTax)
	}
	if usd := EstimateTax(txs, 365, "USD"); usd.EstimatedAnnualTax != 0 {
		t.Errorf("non-NGN statements should not be taxed")
	}
}

func TestRunwayUsesNewestBalanceAndWeightedBurn(t *testing.T) {
	jan := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	// Newest-first statement: Mar, Feb, Jan (all full months)
	txs := []models.DecodedTransaction{
		debit("Rent", 300_000, jan.AddDate(0, 2, 30)),
		debit("Rent", 100_000, jan.AddDate(0, 1, 0)),
		debit("Rent", 100_000, jan),
	}
	txs[0].Balance = 900_000
	txs[2].Balance = 5_000_000

	s := CalculateRunway(txs, 89)
	if s.CurrentBalance != 900_000 {
		t.Errorf("closing balance should come from the newest row, got %.2f", s.CurrentBalance)
	}
	if s.WeightedMonthlyBurn <= s.AvgMonthlyBurn {
		t.Errorf("weighted burn %.2f should exceed flat %.2f when spend is rising", s.WeightedMonthlyBurn, s.AvgMonthlyBurn)
	}
}

func TestApplyContactLabels(t *testing.T) {
	txs := []models.DecodedTransaction{
		debit("Transfer to NGOZI AMAKA OBI | OPay | 1560803617", 2000, t0),
		debit("Transfer to  BELLO RUKAYAT ASABI | MONIE POINT | 7752139889", 1000, t0),
		debit("Transfer to SAMUEL DAVID EKPO | OPay | 111", 700, t0),
	}
	txs[0].Category, txs[0].Flags.IsUrgent2k = "Family & Favors", true
	txs[1].Category = "Market & Small Vendors"

	// Label order and partial names are fine; "Rukayat Bello" is my aunt, "obi ngozi" sells me provisions.
	ApplyContactLabels(txs, []string{"obi ngozi"}, []string{"Rukayat Bello"}, "Market & Small Vendors")

	if txs[0].Category != "Market & Small Vendors" || txs[0].Flags.IsUrgent2k || txs[0].Confidence != 1 {
		t.Errorf("vendor label not applied: %+v", txs[0])
	}
	if txs[1].Category != "Family & Favors" || txs[1].Confidence != 1 {
		t.Errorf("family label should override the Moniepoint vendor guess: %+v", txs[1])
	}
	if txs[2].Confidence == 1 {
		t.Errorf("unlabelled contact must be untouched")
	}
}

func TestGTBankTransferToOwnOPayIsSelfTransfer(t *testing.T) {
	txs := []models.DecodedTransaction{
		debit("000013260808192154000007310610 NIP TRANSFER TO OPAY - CHINEDU OLUMIDE BALOGUN", 102000, t0),
		credit("TRANSFER BETWEEN CUSTOMERS 1000042608-TRANSFER FROM CHINEDU OLUMIDE BALOGUN-OPAY-CHINEDU", 30000, t0),
		debit("NIP TRANSFER TO OPAY - FAVOUR IFEOLUWA BALOGUN", 5000, t0), // sibling
	}
	// GTBank filename order: surname first
	DetectSelfTransfers(txs, "BALOGUN CHINEDU OLUMIDE")
	if !txs[0].Flags.IsSelfTransfer || !txs[1].Flags.IsSelfTransfer {
		t.Errorf("transfers to/from own OPay should be self-transfers: %v %v", txs[0].Flags.IsSelfTransfer, txs[1].Flags.IsSelfTransfer)
	}
	if txs[2].Flags.IsSelfTransfer {
		t.Errorf("sibling sharing a surname must not be a self-transfer")
	}
}

func TestDetectFeeVAT(t *testing.T) {
	fee := debit("Commission on NIP Transfer CHARGES", 50, t0)
	fee.Flags.IsBankFee, fee.Confidence = true, 0.99
	vat := debit("GTW", 3.75, t0)
	vat.Confidence = 0.7
	other := debit("GTW", 3.75, t0.AddDate(0, 0, 3)) // not after a fee
	other.Confidence = 0.7
	txs := []models.DecodedTransaction{fee, vat, other}

	DetectFeeVAT(txs)
	if !txs[1].Flags.IsBankFee || txs[1].Category != "Bank Fees & Charges" {
		t.Errorf("₦3.75 after a ₦50 fee should be VAT, got %q", txs[1].Category)
	}
	if txs[2].Flags.IsBankFee {
		t.Errorf("a small debit not following a fee must be left alone")
	}
}

func p2p(name, bank string, amt float64, at time.Time, isCredit bool) models.DecodedTransaction {
	desc := "Transfer to " + name + " | " + bank + " | 1234567890"
	tx := debit(desc, amt, at)
	if isCredit {
		tx = credit("Transfer from "+name+" | "+bank+" | 1234567890", amt, at)
	}
	tx.Counterparty, tx.DecisionBasis, tx.Category, tx.Confidence = name, "name", "Family & Favors", 0.88
	return tx
}

func TestScorePeople(t *testing.T) {
	txs := []models.DecodedTransaction{
		// Sibling: shares surname, round amounts, sends money back -> family
		p2p("FAVOUR IFEOLUWA BALOGUN", "OPay", 5000, t0, false),
		p2p("FAVOUR IFEOLUWA BALOGUN", "OPay", 2000, t0.AddDate(0, 0, 9), false),
		p2p("FAVOUR IFEOLUWA BALOGUN", "OPay", 1000, t0.AddDate(0, 0, 12), true),
		// Relative on Moniepoint with odd amounts: shares surname -> still definitively family, not review
		p2p("KABIR BALOGUN", "Moniepoint", 1375, t0, false),
		// Trader on OPay (not Moniepoint): odd amounts, one-way -> vendor
		p2p("NGOZI AMAKA OBI", "OPay", 1050, t0, false),
		p2p("NGOZI AMAKA OBI", "OPay", 3810, t0.AddDate(0, 0, 4), false),
		p2p("NGOZI AMAKA OBI", "OPay", 2350, t0.AddDate(0, 0, 8), false),
		// One round transfer, nothing else -> unsure, goes to review
		p2p("SAMUEL DAVID EKPO", "OPay", 2000, t0, false),
	}
	review := ScorePeople(txs, "CHINEDU OLUMIDE BALOGUN")

	if txs[0].Category != "Family & Favors" || txs[0].DecisionBasis != "person" {
		t.Errorf("sibling should be family by person signals: %q / %q", txs[0].Category, txs[0].DecisionBasis)
	}
	if txs[3].Category != "Family & Favors" || txs[3].DecisionBasis != "person" {
		t.Errorf("relative sharing surname should be definitively family: %q / %q", txs[3].Category, txs[3].DecisionBasis)
	}
	if txs[4].Category != "Market & Small Vendors" || txs[4].Flags.IsUrgent2k {
		t.Errorf("odd-amount one-way payee should be a vendor: %q", txs[4].Category)
	}
	found := false
	for _, r := range review {
		if r.Name == "Samuel David Ekpo" && r.Guess == "unsure" {
			found = true
		}
	}
	if !found {
		t.Errorf("weak-signal person should be sent for review, got %+v", review)
	}
}

func TestPaymentLabelOverridesPerson(t *testing.T) {
	tx := p2p("FAVOUR IFEOLUWA BALOGUN", "OPay", 4500, t0, false)
	tx.Date = "03-Aug-2026"
	txs := []models.DecodedTransaction{tx}
	ApplyContactLabels(txs, nil, []string{"Favour Balogun"}, "Market & Small Vendors")
	ApplyPaymentLabels(txs, []string{"03-Aug-2026|4500|Favour Balogun|purchase"}, "Market & Small Vendors")
	if txs[0].Category != "Market & Small Vendors" || txs[0].DecisionBasis != "payment_label" {
		t.Errorf("per-payment label must beat the person label: %q / %q", txs[0].Category, txs[0].DecisionBasis)
	}
}

func TestTotalLeakBurnCountsEachTransactionOnce(t *testing.T) {
	now := time.Now()
	mk := func(id string, at time.Time) models.DecodedTransaction {
		return models.DecodedTransaction{ID: id, Merchant: "Chowdeck Delivery", Amount: -8000, Debit: 8000, ParsedDate: at,
			Flags: models.TransactionFlags{IsMidnightLeak: true}}
	}
	txs := []models.DecodedTransaction{mk("a", now), mk("b", now.Add(5*time.Minute))}
	report := GenerateAudit(txs, "Test", "s", "csv", 0, now, "")
	if report.CashLeaks.TotalLeakBurn != 16000 {
		t.Errorf("expected 16000 total leak burn, got %f", report.CashLeaks.TotalLeakBurn)
	}
}

func TestSelfTransferNameVariants(t *testing.T) {
	holder := nameTokens("JOSEPH ADEDEJI ADEWUNMI")
	for _, name := range []string{"MR JOSEPH ADEWUNMI", "JOSEPHADEDEJI ADEWUNMI", "ADEWUNMI, ADEDEJI JOSEPH"} {
		if !isSameName(holder, name) {
			t.Errorf("%q should be the holder", name)
		}
	}
	for _, name := range []string{"EUNICE IFEOLUWA ADEWUNMI", "MR ADEWUNMI"} {
		if isSameName(holder, name) {
			t.Errorf("%q is a relative, not the holder", name)
		}
	}
}

func TestMerchantCollectionAccountIsNotSelfTransfer(t *testing.T) {
	now := time.Now()
	txs := []models.DecodedTransaction{
		debit("Transfer to SHUTTLERSMETR/JOSEPH ADEWUNMI | Wema Bank | 8000****11", 1500, now),
		debit("NIP TRANSFER TO OPAY - JOSEPH ADEDEJI ADEWUNMI", 5000, now),
	}
	DetectSelfTransfers(txs, "JOSEPH ADEDEJI ADEWUNMI")
	if txs[0].Flags.IsSelfTransfer {
		t.Error("payment to Shuttlers' account in the holder's name is not a self transfer")
	}
	if !txs[1].Flags.IsSelfTransfer {
		t.Error("transfer to the holder's own OPay account is a self transfer")
	}
}

func TestOrganisationsLeftOutOfPeopleReview(t *testing.T) {
	for _, name := range []string{"FCMB PUBLIC OFFER", "JUMIA PAY NGN", "BIGG JOSH SUPER STORE"} {
		if !isOrganisation(name) {
			t.Errorf("%q should be treated as an organisation", name)
		}
	}
	if isOrganisation("OCHOWOCHE SOLOMON JONAH") {
		t.Error("a person's name is not an organisation")
	}
}

func TestDuplicatesNeedMinutesNotHours(t *testing.T) {
	day := time.Date(2025, 2, 22, 7, 23, 0, 0, time.UTC)
	txs := []models.DecodedTransaction{
		debit("Mobile Data | Glo", 270, day),
		debit("Mobile Data | Glo", 270, day.Add(3*time.Hour)), // same plan bought again later
		debit("Mobile Data | Glo", 270, day.Add(3*time.Hour+2*time.Minute)),
	}
	report, _ := AnalyzeDuplicates(txs)
	if report.Count != 1 {
		t.Errorf("only the purchase two minutes after another is a likely double charge, got %d", report.Count)
	}
}

func TestUnsurePersonIsNotUrgent2kLeak(t *testing.T) {
	now := time.Now()
	var txs []models.DecodedTransaction
	for i, amt := range []float64{3000, 5000} { // round amounts only: +1, below the family threshold
		tx := debit("Transfer to OCHOWOCHE SOLOMON JONAH | OPay | 8160****45", amt, now.Add(time.Duration(i)*time.Hour))
		tx.Counterparty, tx.DecisionBasis, tx.Category, tx.Confidence = "OCHOWOCHE SOLOMON JONAH", "name", "Family & Favors", 0.8
		tx.Flags.IsUrgent2k = true
		txs = append(txs, tx)
	}
	review := ScorePeople(txs, "JOSEPH ADEDEJI ADEWUNMI")
	if len(review) != 1 || review[0].Guess != "unsure" {
		t.Fatalf("expected one unsure review, got %+v", review)
	}
	for _, tx := range txs {
		if tx.Flags.IsUrgent2k {
			t.Error("payments to an unsure person should not count as Urgent 2k")
		}
	}
}

func TestMarkUncategorized(t *testing.T) {
	unknownPerson := p2p("SAMUEL DAVID EKPO", "OPay", 2000, t0, false) // basis "name", Family default
	unknownPerson.Flags.IsUrgent2k = true
	unknownMemo := debit("GTW", 3.75, t0)
	unknownMemo.Category, unknownMemo.Confidence = "Personal Discretionary", 0.7
	decided := p2p("FAVOUR IFEOLUWA BALOGUN", "OPay", 5000, t0, false)
	decided.DecisionBasis = "person"
	txs := []models.DecodedTransaction{unknownPerson, unknownMemo, decided}

	r := MarkUncategorized(txs)
	if txs[0].Category != UncategorizedCategory || txs[0].Flags.IsUrgent2k {
		t.Errorf("unsettled person should be Uncategorized and leave Urgent 2k: %q", txs[0].Category)
	}
	if txs[1].Category != UncategorizedCategory {
		t.Errorf("unrecognised memo should be Uncategorized: %q", txs[1].Category)
	}
	if txs[2].Category != "Family & Favors" {
		t.Errorf("decided rows must be left alone: %q", txs[2].Category)
	}
	if r.Count != 2 || r.Total != 2003.75 {
		t.Errorf("report: %+v", r)
	}
}

func TestBuildFindingsRanksAndSkipsSmall(t *testing.T) {
	leaks := models.CashLeaks{}
	leaks.Urgent2k.TotalAmount, leaks.Urgent2k.Count = 852000, 163
	leaks.DuplicateDebits.TotalPotentialRefund, leaks.DuplicateDebits.Count = 17640, 13
	leaks.MidnightLeak.TotalAmount = 400 // below the threshold: noise
	got := BuildFindings("NGN", leaks, models.ReversalsReport{}, models.NigeriaInsights{})
	if len(got) != 2 || got[0].Key != "urgent_2k" || got[1].Key != "duplicates" {
		t.Fatalf("want urgent_2k then duplicates, got %+v", got)
	}
	if got[0].Action == "" {
		t.Error("every finding needs an action")
	}
}

func TestMarkUngraded(t *testing.T) {
	g := models.FinancialGrade{Grade: "B", Score: 72, FocusTip: "save more"}
	MarkUngraded(&g, "reason")
	if g.Grade != "N/A" || g.Score != 0 || g.FocusTip != "" || !g.LowConfidence || g.Note != "reason" {
		t.Errorf("ungraded state not applied: %+v", g)
	}
}
