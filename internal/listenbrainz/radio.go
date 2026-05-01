package listenbrainz

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
)

// GetRadioByTags returns recordings matching given MusicBrainz tags.
// operator: "AND" or "OR" (default "OR")
// popRange: [min, max] popularity 0-100
func (c *Client) GetRadioByTags(ctx context.Context, tags []string, mode string, popRange [2]int) ([]RadioTrack, error) {
	var allTracks []RadioTrack
	for _, tag := range tags {
		tracks, err := c.getRadioByTag(ctx, tag, mode, popRange)
		if err != nil {
			slog.Debug("lb radio tag failed", "tag", tag, "error", err)
			continue
		}
		allTracks = append(allTracks, tracks...)
	}
	return allTracks, nil
}

func (c *Client) getRadioByTag(ctx context.Context, tag, mode string, popRange [2]int) ([]RadioTrack, error) {
	prompt := "tag:(" + tag + ")"
	params := url.Values{}
	params.Set("prompt", prompt)
	params.Set("mode", mode)
	if popRange[0] > 0 || popRange[1] > 0 {
		params.Set("pop_begin", strconv.Itoa(popRange[0]))
		params.Set("pop_end", strconv.Itoa(popRange[1]))
	}
	reqURL := baseURL + "/1/explore/lb-radio?" + params.Encode()

	body, err := c.doGet(ctx, reqURL)
	if err != nil {
		return nil, err
	}

	var resp RadioResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return resp.Payload.Jspf.Playlist.Track, nil
}

// GetCFRecommendations returns collaborative filtering recommendations for a user.
// Requires imported listening history.
func (c *Client) GetCFRecommendations(ctx context.Context, count int) ([]CFRecommendation, error) {
	// GET /1/cf/recommendation/user/{username}/recording?count={count}
	// Call doGet, parse CFResponse, return .Payload.MBIDs
	url := baseURL + "/1/cf/recommendation/user/" + c.username + "/recording?count=" + strconv.Itoa(count)
	body, err := c.doGet(ctx, url)
	if err != nil {
		return nil, err
	}
	
	var cfResponse CFResponse
	if err := json.Unmarshal(body, &cfResponse); err != nil {
		return nil, err
	}
	
	return cfResponse.Payload.MBIDs, nil
}

// Suppress unused import warnings — used in TODO implementations
var (
	_ = fmt.Sprintf
	_ = strconv.Itoa
	_ = strings.Join
	_ = json.Unmarshal
)
