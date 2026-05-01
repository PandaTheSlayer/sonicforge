package listenbrainz

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	baseURL   = "https://api.listenbrainz.org"
	userAgent = "sonicforge/0.1.0"
)

type Client struct {
	httpClient *http.Client
	userToken  string
	username   string
}

func NewClient(userToken, username string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		userToken:  userToken,
		username:   username,
	}
}

// doGet makes an authenticated GET request to ListenBrainz API.
// Handles rate limiting via response headers.
func (c *Client) doGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+c.userToken)
	req.Header.Set("User-Agent", userAgent)

	slog.Debug("listenbrainz request", "url", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("listenbrainz: %w", err)
	}
	defer resp.Body.Close()

	// TODO: read X-RateLimit-Remaining and X-RateLimit-Reset-In headers
	// If remaining == 0, sleep for Reset-In seconds before returning

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("listenbrainz: reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("listenbrainz %s: %d %s", url, resp.StatusCode, string(body))
	}

	return body, nil
}

// ResolveRecording resolves a MusicBrainz recording ID to artist + track names.
func (c *Client) ResolveRecording(ctx context.Context, mbid string) (*RecordingMetadata, error) {
	// TODO: GET /1/metadata/recording/{mbid}?inc=artist+release+tag
	// Parse RecordingMetadata, return it
	return nil, nil
}
