package lastfm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (c *Client) GetTopArtists(ctx context.Context, username string, period string, limit int) ([]Artist, error) {
	resp, err := c.call(ctx, "user.getTopArtists", map[string]string{
		"user":   username,
		"period": period,
		"limit":  strconv.Itoa(limit),
	})
	if err != nil {
		return nil, err
	}

	var result TopArtistsResponse
	err = json.Unmarshal(resp, &result)
	if err != nil {
		return nil, err
	}

	return result.TopArtists.Artist, nil
}

func (c *Client) GetTopTags(ctx context.Context, artistName string) ([]Tag, error) {
	cacheKey := "toptags:" + artistName
	var cached []Tag
	if c.cache != nil && c.cache.Get(cacheKey, 30*24*time.Hour, &cached) {
		return cached, nil
	}

	resp, err := c.call(ctx, "artist.getTopTags", map[string]string{
		"artist": artistName,
	})
	if err != nil {
		return nil, err
	}

	var result TopTagsResponse
	err = json.Unmarshal(resp, &result)
	if err != nil {
		return nil, err
	}

	if c.cache != nil {
		c.cache.Set(cacheKey, result.TopTags.Tag)
	}

	return result.TopTags.Tag, nil
}

func (c *Client) GetSimilar(ctx context.Context, artistName string, limit int) ([]SimilarArtist, error) {
	cacheKey := "similar:" + artistName + ":" + strconv.Itoa(limit)
	var cached []SimilarArtist
	if c.cache != nil && c.cache.Get(cacheKey, 7*24*time.Hour, &cached) {
		return cached, nil
	}

	resp, err := c.call(ctx, "artist.getSimilar", map[string]string{
		"artist": artistName,
		"limit":  strconv.Itoa(limit),
	})
	if err != nil {
		return nil, err
	}

	var result SimilarResponse
	err = json.Unmarshal(resp, &result)
	if err != nil {
		return nil, err
	}

	if c.cache != nil {
		c.cache.Set(cacheKey, result.SimilarArtists.Artists)
	}

	return result.SimilarArtists.Artists, nil
}

func (c *Client) GetTopTracks(ctx context.Context, username, period string, limit int) ([]Track, error) {
	resp, err := c.call(ctx, "user.getTopTracks", map[string]string{
		"user":   username,
		"period": period,
		"limit":  strconv.Itoa(limit),
	})
	if err != nil {
		return nil, err
	}

	var result TopTracksResponse
	err = json.Unmarshal(resp, &result)
	if err != nil {
		return nil, err
	}

	return result.TopTracks.Tracks, nil
}

func (c *Client) BuildProfile(ctx context.Context, username string, topCount int, period string) (*TasteProfile, error) {
	fmt.Fprintf(os.Stderr, "Fetching top artists...")
	topArtists, err := c.GetTopArtists(ctx, username, period, topCount)
	if err != nil {
		fmt.Fprintf(os.Stderr, " error\n")
		return nil, err
	}
	fmt.Fprintf(os.Stderr, " done (%d)\n", len(topArtists))

	var tasteProfile TasteProfile
	var topArtistsConv []ArtistScore
	tasteProfile.KnownArtists = make(map[string]bool)
	for _, a := range topArtists {
		pc, _ := strconv.Atoi(a.Playcount)
		topArtistsConv = append(topArtistsConv, ArtistScore{
			Name:            a.Name,
			Playcount:       pc,
			NormalizedScore: float64(0),
		})
		tasteProfile.KnownArtists[strings.ToLower(a.Name)] = true
	}

	tasteProfile.TopArtists = topArtistsConv

	aggregatedTags := make(map[string]int)
	for i, a := range tasteProfile.TopArtists {
		fmt.Fprintf(os.Stderr, "\rFetching tags... %d/%d", i+1, len(tasteProfile.TopArtists))
		topTags, err := c.GetTopTags(ctx, a.Name)
		if err != nil {
			slog.Debug("Artist skipped", "title", a.Name)
			continue
		}

		var artistTags []string
		for _, t := range topTags {
			aggregatedTags[t.Name] += t.Count
			artistTags = append(artistTags, strings.ToLower(t.Name))
		}
		tasteProfile.TopArtists[i].Tags = artistTags
	}
	fmt.Fprintf(os.Stderr, "\rFetching tags... done (%d unique)\n", len(aggregatedTags))

	var tagScores []TagScore
	for t, w := range aggregatedTags {
		tagScores = append(tagScores, TagScore{
			Tag:              t,
			CumulativeWeight: float64(w),
		})
	}

	sort.Slice(tagScores, func(i, j int) bool {
		return tagScores[i].CumulativeWeight > tagScores[j].CumulativeWeight
	})

	tasteProfile.TopTags = tagScores

	fmt.Fprintf(os.Stderr, "Fetching top tracks...")
	tTr, err := c.GetTopTracks(ctx, username, period, 200)
	if err != nil {
		fmt.Fprintf(os.Stderr, " skipped\n")
		slog.Debug("top tracks skipped", "error", err)
	} else {
		fmt.Fprintf(os.Stderr, " done (%d)\n", len(tTr))
		tasteProfile.KnownTracks = make(map[string]bool)
		for _, tr := range tTr {
			trK := strings.ToLower(tr.Artist.Name) + "::" + strings.ToLower(tr.Name)
			tasteProfile.KnownTracks[trK] = true
		}
	}

	return &tasteProfile, nil
}
