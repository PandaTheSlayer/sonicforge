package engine

import (
	"context"
	"sort"
	"strings"

	"sonicforge/internal/lastfm"
)

func (d *Discoverer) rank(ctx context.Context, candidates []Candidate, profile *lastfm.TasteProfile) []Candidate {
	// phase 1
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].SimilarityScore > candidates[j].SimilarityScore
	})

	limit := 200

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	profileTags := make(map[string]bool)
	for _, t := range profile.TopTags {
		profileTags[strings.ToLower(t.Tag)] = true
	}
	for i, c := range candidates {
		topTags, _ := d.lastfm.GetTopTags(ctx, c.ArtistName)
		for _, t := range topTags {
			candidates[i].Tags = append(candidates[i].Tags, strings.ToLower(t.Name))
		}

		overlap := 0
		for _, t := range candidates[i].Tags {
			if profileTags[t] {
				overlap++
			}
		}

		candidates[i].TagOverlapScore = float64(overlap) / float64(len(profileTags))
		candidates[i].CompositeScore = candidates[i].TagOverlapScore*d.config.Discovery.TagsWeight + c.SimilarityScore*d.config.Discovery.SimilarityWeight
	}
	
	sort.Slice(candidates,  func(i int, j int) bool {
		return candidates[i].CompositeScore > candidates[j].CompositeScore
	})

	if len(candidates) > d.config.Discovery.PlaylistSize {
		candidates = candidates[:d.config.Discovery.PlaylistSize]
	}
	
	return candidates
}
