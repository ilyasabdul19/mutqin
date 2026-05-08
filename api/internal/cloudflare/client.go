// api/internal/cloudflare/client.go
package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is the public Cloudflare v4 API root. Override in tests.
const DefaultBaseURL = "https://api.cloudflare.com/client/v4"

// errorAlreadyExists is the Cloudflare error code returned when a record with
// the same name + type already exists.
const errorAlreadyExists = 81057

// Client talks to the Cloudflare API.
type Client struct {
	token   string
	zoneID  string
	baseURL string
	http    *http.Client
}

// New constructs a Client. baseURL is usually DefaultBaseURL; tests pass an
// httptest server URL.
func New(token, zoneID, baseURL string) *Client {
	return &Client{
		token:   token,
		zoneID:  zoneID,
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// envelope is Cloudflare's standard response shape.
type envelope struct {
	Success  bool             `json:"success"`
	Errors   []envelopeError  `json:"errors"`
	Messages []envelopeError  `json:"messages"`
	Result   *json.RawMessage `json:"result"`
}

type envelopeError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type recordResult struct {
	ID string `json:"id"`
}

// CreateProxiedARecord creates a proxied A record `name → ip` in the configured
// zone. Returns the new record's ID on success. If a record with the same
// name+type already exists, returns ("", nil) — idempotent.
func (c *Client) CreateProxiedARecord(ctx context.Context, name, ip string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"type":    "A",
		"name":    name,
		"content": ip,
		"ttl":     1, // 1 = automatic
		"proxied": true,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/zones/%s/dns_records", c.baseURL, c.zoneID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return "", fmt.Errorf("decode response: %w (status=%d)", err, resp.StatusCode)
	}

	if env.Success {
		if env.Result == nil {
			return "", errors.New("cloudflare: success but missing result")
		}
		var rec recordResult
		if err := json.Unmarshal(*env.Result, &rec); err != nil {
			return "", fmt.Errorf("unmarshal result: %w", err)
		}
		return rec.ID, nil
	}

	for _, e := range env.Errors {
		if e.Code == errorAlreadyExists {
			return "", nil
		}
	}

	if len(env.Errors) > 0 {
		return "", fmt.Errorf("cloudflare error %d: %s", env.Errors[0].Code, env.Errors[0].Message)
	}
	return "", fmt.Errorf("cloudflare request failed (status=%d)", resp.StatusCode)
}
