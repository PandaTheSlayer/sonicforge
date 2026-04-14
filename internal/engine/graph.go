package engine

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"sonicforge/internal/lastfm"
)

func (d *Discoverer) buildSimilarGraph(ctx context.Context, profile *lastfm.TasteProfile) []Candidate {
	var candidates []Candidate
	visited := make(map[string]bool)
	for _, artist := range profile.TopArtists {
		if hasExcludedTag(artist.Tags, d.config.Discovery.ExcludeTags) {
			slog.Debug("skipping artist (excluded tag)", "artist", artist.Name)
			continue
		}
		similar, err := d.lastfm.GetSimilar(ctx, artist.Name, d.config.Discovery.MaxSimilarPerArtist)
		if err != nil {
			slog.Debug("error getting similar artists for ", "title", artist.Name)
			continue
		}

		for _, s := range similar {
			if profile.KnownArtists[strings.ToLower(s.Name)] {
				continue
			}
			similarityScore, err := strconv.ParseFloat(s.Match, 64)
			if err != nil {
				similarityScore = float64(0)
			}
			if similarityScore < d.config.Discovery.MinSimilarity {
				continue
			}
			
			if visited[strings.ToLower(s.Name)] {
				continue
			}
			visited[strings.ToLower(s.Name)] = true
			candidates = append(candidates, Candidate{
				ArtistName:      s.Name,
				Source:          "lastfm-similar",
				SimilarityScore: similarityScore,
			})
		}
	}

	hop1Candidates := make([]Candidate, len(candidates))
	copy(hop1Candidates, candidates)
	if d.config.Discovery.SimilarDepth >= 2 {
		for _, c := range hop1Candidates {
			similar, err := d.lastfm.GetSimilar(ctx, c.ArtistName, d.config.Discovery.MaxSimilarPerArtist)
			if err != nil {
				slog.Debug("error getting similar artists for ", "title", c.ArtistName)
				continue
			}

			for _, s := range similar {
				if profile.KnownArtists[strings.ToLower(s.Name)] {
					continue
				}
				similarityScore, err := strconv.ParseFloat(s.Match, 64)
				if err != nil {
					similarityScore = float64(0)
				}
				if similarityScore < d.config.Discovery.MinSimilarity {
					continue
				}
				
				if visited[strings.ToLower(s.Name)] {
					continue
				}
				visited[strings.ToLower(s.Name)] = true
				candidates = append(candidates, Candidate{
					ArtistName:      s.Name,
					Source:          "lastfm-similar",
					SimilarityScore: similarityScore,
				})
			}
		}
	}

	return candidates
}

func hasExcludedTag(artistTags []string, excludeTags []string) bool {
	for _, at := range artistTags {
		for _, et := range excludeTags {
			if strings.ToLower(at) == strings.ToLower(et) {
				return true
			}
		}
	}
	return false
}
