package engine

import (
	"context"

	"sonicforge/internal/config"
	"sonicforge/internal/lastfm"
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
	config config.Config
}

func NewDiscoverer(lfm *lastfm.Client, cfg config.Config) *Discoverer {
	return &Discoverer{
		lastfm: lfm,
		config: cfg,
	}
}

func (d *Discoverer) Run(ctx context.Context) ([]Candidate, error) { return nil, nil }
func (d *Discoverer) GenerateCandidates(ctx context.Context, profile *lastfm.TasteProfile) ([]Candidate, error) {
	candidates := d.buildSimilarGraph(ctx, profile)
	return candidates, nil
}

func (d *Discoverer) RankAndDedup(ctx context.Context, candidates []Candidate, profile *lastfm.TasteProfile) []Candidate {
	return d.rank(ctx, dedup(candidates), profile)
}
