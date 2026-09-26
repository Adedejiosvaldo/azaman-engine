package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// LayaClient communicates with the local Laya Decision Model sidecar.
type LayaClient struct {
	BaseURL    string
	httpClient *http.Client
	mu         sync.RWMutex
	lastCheck  time.Time
	available  bool
}

// LayaMemoItem represents an unknown or low-confidence memo sent for typed decision.
type LayaMemoItem struct {
	ID      string  `json:"id"`
	Memo    string  `json:"memo"`
	Amount  float64 `json:"amount"`
	RawMemo string  `json:"raw_memo,omitempty"`
	// Hour is the time of day (0-23), nil when the statement has no time column.
	Hour     *int   `json:"hour,omitempty"`
	IsCredit bool   `json:"is_credit"`
	Currency string `json:"currency,omitempty"`
}

// LayaDecision represents the System 1 decision returned by Laya.
type LayaDecision struct {
	ID              string  `json:"id"`
	Merchant        string  `json:"merchant"`
	Category        string  `json:"category"`
	Classification  string  `json:"classification"`
	Confidence      float64 `json:"confidence"`
	IsSilentLeak    bool    `json:"is_silent_leak"`
	IsUrgent2k      bool    `json:"is_urgent_2k"`
	IsTaxDeductible bool    `json:"is_tax_deductible"`
}

// maxResponseBytes bounds how much of a sidecar response is read into RAM.
const maxResponseBytes = 1 << 20

type batchRequest struct {
	Items []LayaMemoItem `json:"items"`
}

type batchResponse struct {
	ExecutionTimeMs float64        `json:"execution_time_ms"`
	Count           int            `json:"count"`
	Results         []LayaDecision `json:"results"`
}

var (
	defaultClient *LayaClient
	once          sync.Once
)

// GetLayaClient returns the singleton Laya sidecar client.
func GetLayaClient() *LayaClient {
	once.Do(func() {
		baseURL := os.Getenv("LAYA_URL")
		if baseURL == "" {
			baseURL = "http://127.0.0.1:8001"
		}

		defaultClient = &LayaClient{
			BaseURL: baseURL,
			httpClient: &http.Client{
				Timeout: 1 * time.Second, // Bounded timeout ensures sub-second responses
			},
		}
	})
	return defaultClient
}

// IsAvailable checks if the local Laya service is running and responsive.
func (c *LayaClient) IsAvailable() bool {
	c.mu.RLock()
	if time.Since(c.lastCheck) < 5*time.Second {
		available := c.available
		c.mu.RUnlock()
		return available
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	req, err := http.NewRequest(http.MethodGet, c.BaseURL+"/health", nil)
	if err != nil {
		c.available = false
		c.lastCheck = time.Now()
		return false
	}

	// 100ms quick probe
	probeClient := &http.Client{Timeout: 150 * time.Millisecond}
	resp, err := probeClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		c.available = false
	} else {
		c.available = true
		_ = resp.Body.Close()
	}
	c.lastCheck = time.Now()
	return c.available
}

// ClassifyBatch queries Laya with a list of ambiguous memos in a single request.
func (c *LayaClient) ClassifyBatch(items []LayaMemoItem) (map[string]LayaDecision, error) {
	if len(items) == 0 {
		return map[string]LayaDecision{}, nil
	}

	reqBody, err := json.Marshal(batchRequest{Items: items})
	if err != nil {
		return nil, err
	}

	url := c.BaseURL + "/api/v1/laya/classify-batch"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// A 4xx/5xx body (e.g. FastAPI validation error) would decode into empty results and pass silently.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("laya sidecar returned %s", resp.Status)
	}

	var bResp batchResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&bResp); err != nil {
		return nil, err
	}

	out := make(map[string]LayaDecision, len(bResp.Results))
	for _, res := range bResp.Results {
		out[res.ID] = res
	}

	return out, nil
}
