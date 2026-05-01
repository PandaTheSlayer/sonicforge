package engine

import (
	"context"
	"log/slog"
	"strings"

	"sonicforge/internal/config"
	"sonicforge/internal/lastfm"
	"sonicforge/internal/listenbrainz"
)

type Candidate struct {
	ArtistName  string
	TrackName   string
	ReleaseName string // optional
	Tags        []string
	Source      string // "lastfm-similar", "lb-cf", "lb-radio-tag", "lb-radio-artist"

	// Scores from different sources (0 if not available from this source)
	SimilarityScore float64
	CFScore         float64
	TagOverlapScore float64
	CompositeScore  float64

	// Metadata
	LastFMArtistURL string
	RecordingMBID   string
	SpotifyURI      string // filled after matching
}

type Discoverer struct {
	lastfm *lastfm.Client
	listenbrainz *listenbrainz.Client
	config config.Config
}

func NewDiscoverer(lfm *lastfm.Client, lb *listenbrainz.Client, cfg config.Config) *Discoverer {
	return &Discoverer{
		lastfm: lfm,
		listenbrainz: lb,
		config: cfg,
	}
}

func (d *Discoverer) Run(ctx context.Context) ([]Candidate, error) { return nil, nil }
func (d *Discoverer) GenerateCandidates(ctx context.Context, profile *lastfm.TasteProfile) ([]Candidate, error) {
	candidates := d.buildSimilarGraph(ctx, profile)

	// LB Radio — tag-based candidates
	if d.listenbrainz != nil {
		var tagNames []string
		for _, t := range profile.TopTags {
			if len(tagNames) >= 5 {
				break
			}
			tagNames = append(tagNames, t.Tag)
		}

		tracks, err := d.listenbrainz.GetRadioByTags(ctx, tagNames, "easy", d.config.Discovery.PopularityRange)
		if err != nil {
			slog.Debug("lb radio skipped", "error", err)
		} else {
			slog.Debug("lb radio returned tracks", "count", len(tracks))
			for _, t := range tracks {
				if profile.KnownArtists[strings.ToLower(t.Creator)] {
					continue
				}
				trKey := strings.ToLower(t.Creator) + "::" + strings.ToLower(t.Title)
				if profile.KnownTracks[trKey] {
					continue
				}
				candidates = append(candidates, Candidate{
					ArtistName:      t.Creator,
					TrackName:       t.Title,
					Source:          "lb-radio-tag",
					SimilarityScore: 0.8, // baseline score so they survive ranking
				})
			}
		}
	}

	return candidates, nil
}

func (d *Discoverer) RankAndDedup(ctx context.Context, candidates []Candidate, profile *lastfm.TasteProfile) []Candidate {
	return d.rank(ctx, dedup(candidates), profile)
}
