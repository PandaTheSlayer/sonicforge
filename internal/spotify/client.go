package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	baseURL   = "https://api.spotify.com/v1"
	userAgent = "sonicforge/0.1.0"
)

type Client struct {
	httpClient  *http.Client
	clientID    string
	accessToken string
}

func NewClient(clientID string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		clientID:   clientID,
	}
}

// doGet makes an authenticated GET request to the Spotify API.
func (c *Client) doGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("spotify %s: %d %s", url, resp.StatusCode, string(body))
	}

	return body, nil
}

// doPost makes an authenticated POST request with JSON body.
func (c *Client) doPost(ctx context.Context, url string, payload any) ([]byte, error) {
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("spotify %s: %d %s", url, resp.StatusCode, string(body))
	}

	return body, nil
}

func (c *Client) Search(ctx context.Context, artistName, trackName string) (*SpotifyTrack, error) {
	params := url.Values{}
	params.Set("q", "track:"+trackName+" artist:"+artistName)
	params.Set("type", "track")
	params.Set("limit", "1")
	body, err := c.doGet(ctx, baseURL+"/search?"+params.Encode())
	if err != nil {
		return nil, err
	}

	var result SearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if len(result.Tracks.Items) == 0 {
		return nil, nil
	}

	return &result.Tracks.Items[0], nil
}

func (c *Client) GetCurrentUser(ctx context.Context) (string, error) {
	body, err := c.doGet(ctx, baseURL+"/me")
	if err != nil {
		return "", err
	}

	var user UserProfile
	if err := json.Unmarshal(body, &user); err != nil {
		return "", err
	}

	return user.ID, nil
}

func (c *Client) CreatePlaylist(ctx context.Context, userID, name, description string) (string, error) {
	payload := map[string]any{
		"name":        name,
		"description": description,
		"public":      false,
	}

	body, err := c.doPost(ctx, baseURL+"/me/playlists", payload)
	if err != nil {
		return "", err
	}

	var result CreatePlaylistResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	return result.ID, nil
}

func (c *Client) AddTracks(ctx context.Context, playlistID string, uris []string) error {
	for i := 0; i < len(uris); i += 100 {
		end := i + 100
		if end > len(uris) {
			end = len(uris)
		}

		payload := map[string]any{
			"uris": uris[i:end],
		}

		_, err := c.doPost(ctx, baseURL+"/playlists/"+playlistID+"/items", payload)
		if err != nil {
			return err
		}
	}

	return nil
}
