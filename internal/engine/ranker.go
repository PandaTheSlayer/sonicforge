package engine

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"sonicforge/internal/lastfm"
)

func (d *Discoverer) rank(ctx context.Context, candidates []Candidate, profile *lastfm.TasteProfile) []Candidate {
	// phase 1: cap pool, but reserve slots for LB Radio candidates so they aren't
	// crowded out by Last.fm similar artists with higher raw similarity scores.
	const limit = 200
	const lbCap = limit / 2 // up to 100 LB Radio slots

	var lbPool, lastfmPool []Candidate
	for _, c := range candidates {
		if strings.HasPrefix(c.Source, "lb-") {
			lbPool = append(lbPool, c)
		} else {
			lastfmPool = append(lastfmPool, c)
		}
	}

	if len(lbPool) > lbCap {
		lbPool = lbPool[:lbCap]
	}

	sort.Slice(lastfmPool, func(i, j int) bool {
		return lastfmPool[i].SimilarityScore > lastfmPool[j].SimilarityScore
	})
	remaining := limit - len(lbPool)
	if remaining > len(lastfmPool) {
		remaining = len(lastfmPool)
	}

	candidates = append(lbPool, lastfmPool[:remaining]...)

	profileTags := make(map[string]bool)
	for _, t := range profile.TopTags {
		profileTags[strings.ToLower(t.Tag)] = true
	}
	for i, c := range candidates {
		fmt.Fprintf(os.Stderr, "\rEnriching candidates... %d/%d", i+1, len(candidates))

		topTags, _ := d.lastfm.GetTopTags(ctx, c.ArtistName)
		for _, t := range topTags {
			candidates[i].Tags = append(candidates[i].Tags, strings.ToLower(t.Name))
		}

		tracks, _ := d.lastfm.GetArtistTopTracks(ctx, c.ArtistName, 10)
		if len(tracks) > 0 {
			candidates[i].TrackName = tracks[0].Name
			artistLower := strings.ToLower(c.ArtistName)
			for _, t := range tracks {
				key := artistLower + "::" + strings.ToLower(t.Name)
				if !profile.KnownTracks[key] {
					candidates[i].TrackName = t.Name
					break
				}
			}
		}

		overlap := 0
		for _, t := range candidates[i].Tags {
			if profileTags[t] {
				overlap++
			}
		}

		candidates[i].TagOverlapScore = float64(overlap) / float64(len(profileTags))
		candidates[i].CompositeScore = candidates[i].TagOverlapScore*d.config.Discovery.TagsWeight + c.SimilarityScore*d.config.Discovery.SimilarityWeight

		// boost/penalty multipliers
		for _, t := range candidates[i].Tags {
			if hasTag(d.config.Discovery.BoostTags, t) {
				candidates[i].CompositeScore *= 2.0
				break
			}
			if hasTag(d.config.Discovery.PenaltyTags, t) {
				candidates[i].CompositeScore *= 0.5
				break
			}
		}
	}
	
	sort.Slice(candidates,  func(i int, j int) bool {
		return candidates[i].CompositeScore > candidates[j].CompositeScore
	})

	if len(candidates) > d.config.Discovery.PlaylistSize {
		candidates = candidates[:d.config.Discovery.PlaylistSize]
	}
	
	return candidates
}

func hasTag(list []string, tag string) bool {
	for _, t := range list {
		if strings.ToLower(t) == tag {
			return true
		}
	}
	return false
}
