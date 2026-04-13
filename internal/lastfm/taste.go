package lastfm

type TasteProfile struct {
	TopArtists   []ArtistScore   // name, playcount, normalized_score
	TopTags      []TagScore      // tag, cumulative weight
	KnownTracks  map[string]bool // "artist::track" → heard
	KnownArtists map[string]bool // artist_name → familiar
}

type ArtistScore struct {
	Name            string
	Playcount       int
	NormalizedScore float64
}

type TagScore struct {
	Tag              string
	CumulativeWeight float64
}
