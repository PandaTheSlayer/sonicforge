package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Image struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type PlayerTrack struct {
	ID         string `json:"id"`
	URI        string `json:"uri"`
	Name       string `json:"name"`
	DurationMs int    `json:"duration_ms"`
	Artists    []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Album struct {
		Name   string  `json:"name"`
		Images []Image `json:"images"`
	} `json:"album"`
}

type CurrentlyPlaying struct {
	IsPlaying  bool         `json:"is_playing"`
	ProgressMs int          `json:"progress_ms"`
	Item       *PlayerTrack `json:"item"`
}

type queueResponse struct {
	Queue []PlayerTrack `json:"queue"`
}

func (c *Client) doRequest(ctx context.Context, method, url string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("User-Agent", userAgent)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("spotify %s %s: %d %s", method, url, resp.StatusCode, string(data))
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	return data, nil
}

func (c *Client) CurrentlyPlaying(ctx context.Context) (*CurrentlyPlaying, error) {
	body, err := c.doRequest(ctx, http.MethodGet, baseURL+"/me/player/currently-playing?additional_types=track", nil)
	if err != nil || body == nil {
		return nil, err
	}

	var result CurrentlyPlaying
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) Queue(ctx context.Context, limit int) ([]PlayerTrack, error) {
	body, err := c.doRequest(ctx, http.MethodGet, baseURL+"/me/player/queue", nil)
	if err != nil || body == nil {
		return nil, err
	}

	var result queueResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if len(result.Queue) > limit {
		result.Queue = result.Queue[:limit]
	}

	return result.Queue, nil
}

func (c *Client) InLibrary(ctx context.Context, uris []string) ([]bool, error) {
	body, err := c.doRequest(ctx, http.MethodGet, libraryURL("/me/library/contains", uris), nil)
	if err != nil {
		return nil, err
	}

	var result []bool
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func libraryURL(path string, uris []string) string {
	params := url.Values{}
	params.Set("uris", strings.Join(uris, ","))
	return baseURL + path + "?" + params.Encode()
}

func (c *Client) SaveToLibrary(ctx context.Context, uris []string) error {
	_, err := c.doRequest(ctx, http.MethodPut, libraryURL("/me/library", uris), nil)
	return err
}

func (c *Client) RemoveFromLibrary(ctx context.Context, uris []string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, libraryURL("/me/library", uris), nil)
	return err
}

func SmallestImage(images []Image, minSize int) string {
	best := ""
	bestWidth := 0
	for _, img := range images {
		if img.Width >= minSize && (best == "" || img.Width < bestWidth) {
			best, bestWidth = img.URL, img.Width
		}
	}
	if best == "" && len(images) > 0 {
		best = images[0].URL
	}
	return best
}

var ErrNoDevice = errors.New("spotify: no active device")

func (c *Client) AddToQueue(ctx context.Context, uri string) error {
	params := url.Values{}
	params.Set("uri", uri)
	_, err := c.doRequest(ctx, http.MethodPost, baseURL+"/me/player/queue?"+params.Encode(), nil)
	return noDevice(err)
}

func (c *Client) Next(ctx context.Context) error {
	_, err := c.doRequest(ctx, http.MethodPost, baseURL+"/me/player/next", nil)
	return noDevice(err)
}

func noDevice(err error) error {
	if err != nil && strings.Contains(err.Error(), ": 404 ") {
		return fmt.Errorf("%w: %v", ErrNoDevice, err)
	}
	return err
}

func (c *Client) Track(ctx context.Context, id string) (*PlayerTrack, error) {
	body, err := c.doRequest(ctx, http.MethodGet, baseURL+"/tracks/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}

	var t PlayerTrack
	if err := json.Unmarshal(body, &t); err != nil {
		return nil, err
	}

	return &t, nil
}
