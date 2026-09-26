package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"azaman-engine/internal/shazam"
)

// Categories assigned by analyzer/matcher code rather than the merchant registry.
var engineAssignedCategories = []string{
	"Family & Favors", "Commercial & Business", "POS Cashout & Agency", "Personal Discretionary",
	"Income & Retainers", "Loan Disbursement", "Internal Transfers", "Reversals & Refunds",
	"Black Tax (Family Support)", "Recurring Obligations", "Ajo & Thrift", "Rent & Housing",
}

// TestCategoriesMatchSidecar keeps sidecar/categories.json in sync with every Go category, so a
// Laya answer always lands in a bucket the insights, fee and business totals know about.
func TestCategoriesMatchSidecar(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "sidecar", "categories.json"))
	if err != nil {
		t.Fatalf("read taxonomy: %v", err)
	}
	var file struct {
		Categories []struct {
			Name string `json:"name"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("parse taxonomy: %v", err)
	}
	known := map[string]bool{}
	for _, c := range file.Categories {
		known[c.Name] = true
	}

	want := append([]string{}, engineAssignedCategories...)
	for _, rule := range shazam.MerchantRegistry {
		want = append(want, rule.Category)
	}
	for _, c := range want {
		if !known[c] {
			t.Errorf("category %q is used by the engine but missing from sidecar/categories.json", c)
		}
	}
}
