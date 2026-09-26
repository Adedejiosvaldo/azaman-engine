package parser

import (
	"regexp"
	"strings"
)

// Supported Bank Identifiers
const (
	BankGTB        = "GTBank"
	BankKuda       = "Kuda Bank"
	BankAccess     = "Access Bank"
	BankMoniepoint = "Moniepoint"
	BankOPay       = "OPay"
	BankPalmPay    = "PalmPay"
	BankZenith     = "Zenith Bank"
	BankFirstBank  = "First Bank"
	BankUBA        = "UBA"
	BankWema       = "Wema Bank (ALAT)"
	BankFidelity   = "Fidelity Bank"
	BankSterling   = "Sterling Bank"
	BankFCMB       = "FCMB"
	BankStanbic    = "Stanbic IBTC"
	BankGeegpay    = "Geegpay USD"
	BankGeneric    = "Generic Bank"
)

// bankSignatures is checked in order. Every statement lists counterparties' banks in its narrations
// ("Transfer to X | OPay"), so specific issuer names come before generic brand words.
var bankSignatures = []struct {
	bank     string
	keywords []string
}{
	{BankGeegpay, []string{"geegpay technologies", "geegpay", "usd wallet"}},
	{BankOPay, []string{"opay digital", "wallet account statement", "owealth", "opay"}},
	{BankPalmPay, []string{"palmpay limited", "palmpay"}},
	{BankKuda, []string{"kuda microfinance", "kuda bank", "money app for peace of mind", "kudabank"}},
	{BankMoniepoint, []string{"moniepoint microfinance", "moniepoint mfb", "moniepoint"}},
	{BankGTB, []string{"guaranty trust", "gtbank", "gtco"}},
	{BankZenith, []string{"zenith bank", "zenithbank"}},
	{BankFirstBank, []string{"first bank of nigeria", "firstbank", "first bank"}},
	{BankUBA, []string{"united bank for africa", "uba plc", "ubagroup"}},
	{BankAccess, []string{"access bank plc", "access bank", "accessbank"}},
	{BankWema, []string{"wema bank", "alat by wema", "alat"}},
	{BankFidelity, []string{"fidelity bank", "fidelitybank"}},
	{BankSterling, []string{"sterling bank", "sterling financial"}},
	{BankFCMB, []string{"first city monument", "fcmb"}},
	{BankStanbic, []string{"stanbic ibtc", "stanbic"}},
}

// headerWindow is the leading slice of the statement that holds the issuer's letterhead.
const headerWindow = 600

// DetectBank inspects the raw text content of a statement (first 4KB - 16KB) to determine bank source.
// The letterhead is checked first so counterparty bank names in narrations do not win.
func DetectBank(sampleText string) string {
	lower := strings.ToLower(sampleText)
	header := lower
	if len(header) > headerWindow {
		header = header[:headerWindow]
	}

	// Letterhead: the bank named first wins (narrations mention other banks further down).
	best, bestPos := "", len(header)+1
	for _, sig := range bankSignatures {
		for _, kw := range sig.keywords {
			if pos := strings.Index(header, kw); pos >= 0 && pos < bestPos {
				best, bestPos = sig.bank, pos
			}
		}
	}
	if best != "" {
		return best
	}

	for _, sig := range bankSignatures {
		for _, kw := range sig.keywords {
			if strings.Contains(lower, kw) {
				return sig.bank
			}
		}
	}

	// GTBank CSV exports have no letterhead but a distinctive column set
	if strings.Contains(lower, "trans date") && strings.Contains(lower, "remarks") && strings.Contains(lower, "reference") {
		return BankGTB
	}
	return BankGeneric
}

var accountHolderRe = regexp.MustCompile(`(?im)^[\s"]*(?:account\s*name|account\s*holder|customer\s*name|name\s*of\s*account)["\s]*[:,]["\s]*([A-Za-z][A-Za-z .,'-]{2,60}?)["\s]*(?:,|$|\s{2,}|account)`)

// ExtractAccountHolder returns the account holder's name from a statement header, if present.
// It is used to recognise transfers to the holder's own accounts at other banks.
func ExtractAccountHolder(sampleText string) string {
	m := accountHolderRe.FindStringSubmatch(sampleText)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(strings.Trim(m[1], ",. "))
}
