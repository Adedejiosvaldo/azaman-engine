package parser

import (
	"strings"
)

// Supported Bank Identifiers
const (
	BankGTB        = "GTBank"
	BankKuda       = "Kuda Bank"
	BankAccess     = "Access Bank"
	BankMoniepoint = "Moniepoint"
	BankOPay       = "OPay"
	BankGeegpay    = "Geegpay USD"
	BankGeneric    = "Generic Bank"
)

// DetectBank inspects the raw text content of a statement (first 4KB - 16KB) to determine bank source.
func DetectBank(sampleText string) string {
	lower := strings.ToLower(sampleText)

	// Check OPay
	if strings.Contains(lower, "opay") || strings.Contains(lower, "owealth") ||
		strings.Contains(lower, "wallet account statement") {
		return BankOPay
	}

	// Check GTBank
	if strings.Contains(lower, "guaranty trust") || strings.Contains(lower, "gtbank") || strings.Contains(lower, "gtco") ||
		(strings.Contains(lower, "trans date") && strings.Contains(lower, "remarks") && strings.Contains(lower, "reference")) {
		return BankGTB
	}

	// Check Kuda
	if strings.Contains(lower, "kuda microfinance") || strings.Contains(lower, "kuda bank") ||
		strings.Contains(lower, "money app for peace of mind") || strings.Contains(lower, "kudabank") {
		return BankKuda
	}

	// Check Moniepoint
	if strings.Contains(lower, "moniepoint mfb") || strings.Contains(lower, "moniepoint microfinance") ||
		strings.Contains(lower, "moniepoint") {
		return BankMoniepoint
	}

	// Check Geegpay
	if strings.Contains(lower, "geegpay") || strings.Contains(lower, "usd wallet") ||
		strings.Contains(lower, "geegpay technologies") {
		return BankGeegpay
	}

	// Check Access Bank
	if strings.Contains(lower, "access bank") || strings.Contains(lower, "access bank plc") ||
		strings.Contains(lower, "accessbank") {
		return BankAccess
	}

	return BankGeneric
}
