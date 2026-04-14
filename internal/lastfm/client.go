package lastfm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"sonicforge/internal/cache"
)

const (
	defaultBaseURL = "https://ws.audioscrobbler.com/2.0/"
	userAgent      = "sonicforge/0.1.0"
)

type Client struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
	limiter    *time.Ticker
	cache      *cache.Cache
}

func NewClient(apiKey string, c *cache.Cache) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		limiter:    time.NewTicker(200 * time.Millisecond), // 5 req/sec
		cache:      c,
	}
}

// Close stops the rate limiter. Call when done with the client.
func (c *Client) Close() {
	c.limiter.Stop()
}

// call makes a rate-limited GET request to the Last.fm API.
// method is the API method (e.g. "user.getTopArtists").
// params are additional query parameters.
// Returns the raw response body or an error (including Last.fm API errors).
func (c *Client) call(ctx context.Context, method string, params map[string]string) ([]byte, error) {
	<-c.limiter.C
	
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("lastfm: parsing base URL: %w", err)
	}

	q := u.Query()
	q.Set("method", method)
	q.Set("api_key", c.apiKey)
	q.Set("format", "json")
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("lastfm: creating request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	slog.Debug("lastfm request", "method", method, "url", u.String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lastfm: %s: %w", method, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lastfm: reading response for %s: %w", method, err)
	}

	var apiErr APIError
	if json.Unmarshal(body, &apiErr); apiErr.Code != 0 {
		return nil, fmt.Errorf("lastfm: %s: %w", method, &apiErr)
	}

	return body, nil
}
