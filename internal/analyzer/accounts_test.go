package analyzer

import (
	"testing"
	"time"

	"azaman-engine/internal/models"
)

func at(day, hour int) time.Time {
	return time.Date(2026, 3, day, hour, 0, 0, 0, time.UTC)
}

func debitTx(id, account, desc string, amount float64, when time.Time) models.DecodedTransaction {
	return models.DecodedTransaction{ID: id, Account: account, RawDescription: desc, CleanMemo: desc,
		Debit: amount, Amount: -amount, ParsedDate: when, Currency: "NGN"}
}

func creditTx(id, account, desc string, amount float64, when time.Time) models.DecodedTransaction {
	return models.DecodedTransaction{ID: id, Account: account, RawDescription: desc, CleanMemo: desc,
		Credit: amount, Amount: amount, ParsedDate: when, Currency: "NGN"}
}

func TestCrossAccountTransferIsMatched(t *testing.T) {
	txs := []models.DecodedTransaction{
		debitTx("a", "GTBank", "NIP/DR/0123/TUNDE BAKARE/", 50000, at(3, 10)),
		creditTx("b", "OPay", "Transfer from TUNDE BAKARE", 50000, at(3, 10)),
		debitTx("c", "OPay", "Chowdeck order", 6000, at(4, 22)),
	}
	if n := MatchCrossAccountTransfers(txs, "Tunde Bakare"); n != 1 {
		t.Fatalf("expected 1 pair, got %d", n)
	}
	if !txs[0].Flags.IsSelfTransfer || !txs[1].Flags.IsSelfTransfer {
		t.Errorf("both legs should be self-transfers")
	}
	if txs[2].Flags.IsSelfTransfer {
		t.Errorf("unrelated spend must not be matched")
	}
}

func TestCrossAccountSkipsOtherPeople(t *testing.T) {
	// Paying a friend from GTBank and a different friend paying you into OPay, same amount, same day.
	txs := []models.DecodedTransaction{
		debitTx("a", "GTBank", "Transfer to SEGUN ALADE", 5000, at(3, 9)),
		creditTx("b", "OPay", "Transfer from AMAKA OBI", 5000, at(3, 15)),
	}
	if n := MatchCrossAccountTransfers(txs, "Tunde Bakare"); n != 0 {
		t.Fatalf("expected no pair, got %d", n)
	}
}

func TestCrossAccountNeedsTwoAccounts(t *testing.T) {
	txs := []models.DecodedTransaction{
		debitTx("a", "GTBank", "to self", 10000, at(3, 9)),
		creditTx("b", "GTBank", "salary", 10000, at(3, 9)),
	}
	if n := MatchCrossAccountTransfers(txs, ""); n != 0 {
		t.Fatalf("single account must not pair, got %d", n)
	}
}

func TestCrossAccountWindow(t *testing.T) {
	txs := []models.DecodedTransaction{
		debitTx("a", "GTBank", "transfer", 20000, at(3, 9)),
		creditTx("b", "Kuda", "transfer", 20000, at(10, 9)), // a week later: not the same transfer
	}
	if n := MatchCrossAccountTransfers(txs, ""); n != 0 {
		t.Fatalf("credit outside window must not pair, got %d", n)
	}
}

func TestGenerateAuditCombinesAccounts(t *testing.T) {
	txs := []models.DecodedTransaction{
		creditTx("s", "GTBank", "Salary ACME LTD", 300000, at(1, 9)),
		debitTx("a", "GTBank", "Transfer to TUNDE BAKARE", 50000, at(3, 10)),
		creditTx("b", "OPay", "Transfer from TUNDE BAKARE", 50000, at(3, 10)),
		debitTx("c", "OPay", "Chowdeck order", 6000, at(4, 14)),
	}
	txs[0].Balance, txs[1].Balance = 300000, 250000
	txs[2].Balance, txs[3].Balance = 50000, 44000

	report := GenerateAudit(txs, "GTBank + OPay", "s1", "csv", 0, time.Now(), "Tunde Bakare")
	if report.Summary.TotalInflow != 300000 {
		t.Errorf("inflow should exclude the transfer leg, got %v", report.Summary.TotalInflow)
	}
	if report.Summary.TotalOutflow != 6000 {
		t.Errorf("outflow should exclude the transfer leg, got %v", report.Summary.TotalOutflow)
	}
	if report.Summary.CurrentBalance != 294000 {
		t.Errorf("balance should add both accounts (250000 + 44000), got %v", report.Summary.CurrentBalance)
	}
	if report.Internal.CrossAccountPairs != 1 {
		t.Errorf("expected 1 cross-account pair, got %d", report.Internal.CrossAccountPairs)
	}

	accounts := []models.AccountSummary{{Label: "GTBank"}, {Label: "OPay"}}
	FillAccountTotals(accounts, report.Transactions)
	if accounts[0].Inflow != 300000 || accounts[0].Outflow != 0 || accounts[0].ClosingBalance != 250000 {
		t.Errorf("GTBank totals wrong: %+v", accounts[0])
	}
	if accounts[1].Outflow != 6000 || accounts[1].ClosingBalance != 44000 {
		t.Errorf("OPay totals wrong: %+v", accounts[1])
	}
}
