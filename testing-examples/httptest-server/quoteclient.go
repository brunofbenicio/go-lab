// Package quoteclient is a client for a hypothetical JSON price API:
//
//	GET {base}/quotes/{symbol}  ->  {"symbol":"ABC","price":12.34,"currency":"USD"}
//
// Its tests run against a real HTTP server on the loopback interface.
package quoteclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Errors returned by Client.Quote.
var (
	ErrNotFound = errors.New("quoteclient: symbol not found")
)

// RateLimitError is returned on HTTP 429. RetryAfter is zero when the server
// did not send a usable Retry-After header.
type RateLimitError struct{ RetryAfter time.Duration }

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("quoteclient: rate limited, retry after %s", e.RetryAfter)
}

// Quote is a price quote.
type Quote struct {
	Symbol   string  `json:"symbol"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
}

// Client talks to the API at BaseURL. A nil HTTP uses http.DefaultClient.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// Quote fetches the current quote for symbol.
func (c *Client) Quote(ctx context.Context, symbol string) (Quote, error) {
	endpoint := c.BaseURL + "/quotes/" + url.PathEscape(symbol)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Quote{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return Quote{}, fmt.Errorf("quote %s: %w", symbol, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		var q Quote
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&q); err != nil {
			return Quote{}, fmt.Errorf("quote %s: decode: %w", symbol, err)
		}
		return q, nil
	case resp.StatusCode == http.StatusNotFound:
		return Quote{}, fmt.Errorf("quote %s: %w", symbol, ErrNotFound)
	case resp.StatusCode == http.StatusTooManyRequests:
		secs, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
		return Quote{}, &RateLimitError{RetryAfter: time.Duration(secs) * time.Second}
	default:
		return Quote{}, fmt.Errorf("quote %s: unexpected status %s", symbol, resp.Status)
	}
}
