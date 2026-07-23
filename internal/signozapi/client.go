// Package signozapi is the read-only client for the SigNoz REST API used by
// the usage cross-reference profiler ("written but never read"). It issues
// only GET requests (product invariant NFR-2).
package signozapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Document is one dashboard or alert rule, kept as raw JSON: SigNoz response
// shapes vary across versions, so the consumer walks the JSON defensively
// instead of binding to a struct.
type Document struct {
	Title string          `json:"title"`
	Raw   json.RawMessage `json:"raw"`
}

// SignozAPI is the read-only surface of the SigNoz REST API TELELENS needs.
type SignozAPI interface {
	Dashboards(ctx context.Context) ([]Document, error)
	Rules(ctx context.Context) ([]Document, error)
}

// ---- live client ----

// Client talks to a live SigNoz API (GET only).
type Client struct {
	BaseURL string
	APIKey  string
	HTTPC   *http.Client
}

// NewClient returns a live client for baseURL (e.g. http://localhost:8080).
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTPC:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) get(ctx context.Context, path string) ([]Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if c.APIKey != "" {
		req.Header.Set("SIGNOZ-API-KEY", c.APIKey)
	}
	resp, err := c.HTTPC.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", path, resp.StatusCode)
	}
	return parseListResponse(body)
}

// parseListResponse handles the {"status":"success","data":[...]} envelope and
// a few version-dependent nestings ({"data":{"rules":[...]}} etc.).
func parseListResponse(body []byte) ([]Document, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || len(envelope.Data) == 0 {
		envelope.Data = body // no envelope: treat the body as the payload
	}
	items := extractList(envelope.Data)
	docs := make([]Document, 0, len(items))
	for _, raw := range items {
		var t struct {
			Title string `json:"title"`
			Alert string `json:"alert"`
		}
		_ = json.Unmarshal(raw, &t)
		title := t.Title
		if title == "" {
			title = t.Alert
		}
		docs = append(docs, Document{Title: title, Raw: raw})
	}
	return docs, nil
}

// extractList finds the first JSON array in data: either data itself, or the
// first array-valued field of a wrapping object (e.g. {"rules": [...]}).
func extractList(data json.RawMessage) []json.RawMessage {
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err == nil {
		for _, v := range obj {
			if err := json.Unmarshal(v, &arr); err == nil && len(arr) > 0 {
				return arr
			}
		}
	}
	return nil
}

func (c *Client) Dashboards(ctx context.Context) ([]Document, error) {
	return c.get(ctx, "/api/v1/dashboards")
}

func (c *Client) Rules(ctx context.Context) ([]Document, error) {
	return c.get(ctx, "/api/v1/rules")
}

// ---- fixture client ----

// FixtureAPI serves dashboards/rules from fixtures/dashboards.json and
// fixtures/rules.json (arrays of dashboard / rule objects).
type FixtureAPI struct{ Dir string }

func (f *FixtureAPI) load(name string) ([]Document, error) {
	data, err := os.ReadFile(filepath.Join(f.Dir, name+".json"))
	if err != nil {
		return nil, fmt.Errorf("load fixture %s: %w", name, err)
	}
	return parseListResponse(data)
}

func (f *FixtureAPI) Dashboards(context.Context) ([]Document, error) { return f.load("dashboards") }
func (f *FixtureAPI) Rules(context.Context) ([]Document, error)      { return f.load("rules") }
