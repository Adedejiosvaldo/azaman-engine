// memo-eval scores how well the engine categorises Nigerian bank memos, with and without the AI.
//
//	go run ./cmd/memo-eval -laya http://127.0.0.1:8001            # all rows
//	go run ./cmd/memo-eval -laya http://127.0.0.1:8001 -split dev -v
//
// It runs each labelled memo in testdata/memo_eval.jsonl through the same code as a real audit
// (rules, then the AI pass, then "Uncategorized" marking) and compares the final category with the
// label. Two modes:
//
//	statement: all memos as one audit, so the per-audit AI limits apply (what users get)
//	each:      every memo as its own audit (how good the AI is when it gets to answer)
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"azaman-engine/internal/ai"
	"azaman-engine/internal/analyzer"
	"azaman-engine/internal/models"
	"azaman-engine/internal/shazam"
)

type row struct {
	Memo   string   `json:"memo"`
	Amount float64  `json:"amount"`
	Hour   *int     `json:"hour"`
	Want   string   `json:"want"`
	Alt    []string `json:"alt"`
	Kind   string   `json:"kind"`
	Split  string   `json:"split"`
}

func (r row) ok(category string) bool {
	if category == r.Want {
		return true
	}
	for _, a := range r.Alt {
		if category == a {
			return true
		}
	}
	return false
}

func load(path, split string) []row {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	var rows []row
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r row
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			log.Fatal(err)
		}
		if split == "all" || r.Split == split {
			rows = append(rows, r)
		}
	}
	return rows
}

func decode(rows []row) []models.DecodedTransaction {
	txs := make([]models.DecodedTransaction, len(rows))
	for i, r := range rows {
		when := time.Date(2026, 3, 1+i%28, 0, 0, 0, 0, time.UTC)
		if r.Hour != nil {
			when = when.Add(time.Duration(*r.Hour)*time.Hour + 7*time.Minute)
		}
		txs[i] = shazam.MatchAndDecode(models.RawTransaction{
			Description: r.Memo, Debit: r.Amount, ParsedDate: when, Currency: "NGN",
		})
	}
	return txs
}

// run returns the final category for every row, plus the AI pass stats.
func run(rows []row, client ai.Classifier, mode string) ([]string, ai.PassStats) {
	out := make([]string, len(rows))
	var total ai.PassStats
	finish := func(txs []models.DecodedTransaction, offset int) {
		if client != nil {
			s := ai.ApplyDecisions(txs, client, "")
			total.Candidates += s.Candidates
			total.Sent += s.Sent
			total.Changed += s.Changed
		}
		analyzer.MarkUncategorized(txs)
		for i, tx := range txs {
			out[offset+i] = tx.Category
		}
	}
	if mode == "statement" {
		finish(decode(rows), 0)
		return out, total
	}
	for i := range rows {
		finish(decode(rows[i:i+1]), i)
	}
	return out, total
}

// marginFilter drops model answers whose margin is below min, as the sidecar would with
// LAYA_MIN_MARGIN=min. Run the sidecar with LAYA_MIN_MARGIN=0 and sweep -margin here.
type marginFilter struct {
	inner ai.Classifier
	min   float64
}

func (m marginFilter) IsAvailable() bool { return m.inner.IsAvailable() }

func (m marginFilter) ClassifyBatch(items []ai.LayaMemoItem) (map[string]ai.LayaDecision, error) {
	out, err := m.inner.ClassifyBatch(items)
	for id, d := range out {
		if d.Margin != nil && *d.Margin < m.min {
			d.Confidence = 0
			out[id] = d
		}
	}
	return out, err
}

type score struct{ right, total int }

func (s score) String() string {
	if s.total == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%5.1f%% (%d/%d)", 100*float64(s.right)/float64(s.total), s.right, s.total)
}

func main() {
	data := flag.String("data", "testdata/memo_eval.jsonl", "labelled memos")
	split := flag.String("split", "all", "dev, test or all")
	layaURL := flag.String("laya", "", "Laya sidecar URL; empty scores the rules only")
	timeout := flag.Duration("timeout", 30*time.Second, "AI request timeout (generous: this measures quality, not speed)")
	verbose := flag.Bool("v", false, "list every row the AI changed")
	margin := flag.Float64("margin", 0, "ignore model answers whose top-vs-runner-up margin is below this")
	flag.Parse()

	rows := load(*data, *split)
	var client ai.Classifier
	if *layaURL != "" {
		c := ai.NewLayaClient(*layaURL, *timeout)
		if !c.IsAvailable() {
			log.Fatalf("no Laya sidecar answering at %s", *layaURL)
		}
		client = marginFilter{inner: c, min: *margin}
	}

	fmt.Printf("%d memos (split=%s, margin=%.2f)\n\n", len(rows), *split, *margin)
	for _, mode := range []string{"statement", "each"} {
		rulesOnly, _ := run(rows, nil, mode)
		withAI := rulesOnly
		var stats ai.PassStats
		if client != nil {
			start := time.Now()
			withAI, stats = run(rows, client, mode)
			fmt.Printf("[%s] AI time %s\n", mode, time.Since(start).Round(time.Millisecond))
		}

		byKind := map[string][2]score{}
		var all [2]score
		fixed, broken, personTouched := 0, 0, 0
		var changes []string
		for i, r := range rows {
			if r.Kind == "person" {
				if withAI[i] != rulesOnly[i] {
					personTouched++
					changes = append(changes, fmt.Sprintf("  PERSON TOUCHED  %-55.55s %s -> %s", r.Memo, rulesOnly[i], withAI[i]))
				}
				continue
			}
			k := byKind[r.Kind]
			for j, got := range []string{rulesOnly[i], withAI[i]} {
				k[j].total++
				all[j].total++
				if r.ok(got) {
					k[j].right++
					all[j].right++
				}
			}
			byKind[r.Kind] = k
			if withAI[i] != rulesOnly[i] {
				tag := "same"
				switch {
				case !r.ok(rulesOnly[i]) && r.ok(withAI[i]):
					fixed++
					tag = "FIXED "
				case r.ok(rulesOnly[i]) && !r.ok(withAI[i]):
					broken++
					tag = "BROKE "
				}
				changes = append(changes, fmt.Sprintf("  %s %-55.55s %s -> %s (want %s)", tag, r.Memo, rulesOnly[i], withAI[i], r.Want))
			}
		}

		fmt.Printf("[%s]\n  %-10s %-22s %-22s\n", mode, "kind", "rules only", "rules + AI")
		kinds := make([]string, 0, len(byKind))
		for k := range byKind {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		for _, k := range kinds {
			fmt.Printf("  %-10s %-22s %-22s\n", k, byKind[k][0], byKind[k][1])
		}
		fmt.Printf("  %-10s %-22s %-22s\n", "ALL", all[0], all[1])
		if client != nil {
			fmt.Printf("  AI: unsure=%d sent=%d changed=%d | fixed=%d broke=%d | person rows touched=%d\n",
				stats.Candidates, stats.Sent, stats.Changed, fixed, broken, personTouched)
		}
		if *verbose && len(changes) > 0 {
			fmt.Println(strings.Join(changes, "\n"))
		}
		fmt.Println()
	}
}
