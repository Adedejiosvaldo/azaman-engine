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
