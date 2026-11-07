package ai

import (
	"regexp"
	"strings"
)

var (
	// Account numbers, references, terminal IDs, phone numbers: noise to the model, and personal.
	digitsRe  = regexp.MustCompile(`[0-9*]{3,}`)
	nonWordRe = regexp.MustCompile(`[^a-z0-9&' ]+`)
	// Bank names made of everyday words are removed only as whole names.
	bankPhraseRe = regexp.MustCompile(`\b(monie ?point|first ?bank|union bank|access bank|guaranty trust( bank)?|unity bank|fidelity bank|sterling bank|wema bank|zenith bank|keystone bank|polaris bank|providus bank|kuda mfb|opay digital services)\b`)
)

// Words that say how money moved or which bank carried it, not what it bought. Bank names in
// particular never imply a category, and they mislead a model ("Carbon" is a bank here).
var noiseWords = map[string]bool{
	"transfer": true, "trf": true, "to": true, "from": true, "nip": true, "dr": true, "cr": true,
	"pos": true, "debit": true, "credit": true, "web": true, "purchase": true, "payment": true,
	"pmt": true, "card": true, "ref": true, "via": true, "ng": true, "lagos": true, "abuja": true,
	"paystack": true, "flutterwave": true, "interswitch": true, "paycom": true,
	"opay": true, "palmpay": true, "kuda": true, "moniepoint": true, "carbon": true,
	"gtbank": true, "gtb": true, "uba": true, "firstbank": true, "fcmb": true, "stanbic": true,
	"ecobank": true, "jaiz": true, "mfb": true, "microfinance": true, "bank": true,
}

// CleanForAI reduces a memo to the words that say what the money was for. Numbers, bank names and
// transfer boilerplate are removed, and so is the account holder's own name, so neither account
// numbers nor the user's identity reach the model. Returns "" when nothing useful is left.
func CleanForAI(text, holder string) string {
	lower := strings.ToLower(text)
	lower = digitsRe.ReplaceAllString(lower, " ")
	lower = nonWordRe.ReplaceAllString(lower, " ")
	lower = bankPhraseRe.ReplaceAllString(lower, " ")

	own := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(holder)) {
		if len(w) >= 3 {
			own[w] = true
		}
	}

	var kept []string
	for _, w := range strings.Fields(lower) {
		if noiseWords[w] || own[w] || len(w) < 2 {
			continue
		}
		kept = append(kept, w)
	}
	return strings.Join(kept, " ")
}
