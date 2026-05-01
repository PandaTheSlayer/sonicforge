package listenbrainz

// CF Recommendations response
type CFResponse struct {
	Payload struct {
		LastUpdated    int                `json:"last_updated"`
		Entity         string             `json:"entity"`
		MBIDs          []CFRecommendation `json:"mbids"`
		UserName       string             `json:"user_name"`
		Count          int                `json:"count"`
		TotalMBIDCount int                `json:"total_mbid_count"`
		Offset         int                `json:"offset"`
	} `json:"payload"`
}

type CFRecommendation struct {
	RecordingMBID string  `json:"recording_mbid"`
	Score         float64 `json:"score"`
}

// Metadata resolution response
type RecordingMetadata struct {
	ArtistCreditName string   `json:"artist_credit_name"`
	ArtistMBIDs      []string `json:"artist_mbids"`
	RecordingMBID    string   `json:"recording_mbid"`
	RecordingName    string   `json:"recording_name"`
	ReleaseMBID      string   `json:"release_mbid"`
	ReleaseName      string   `json:"release_name"`
}

// LB Radio response — tag-based and artist-seeded (JSPF format)
type RadioResponse struct {
	Payload struct {
		Jspf struct {
			Playlist struct {
				Track []RadioTrack `json:"track"`
			} `json:"playlist"`
		} `json:"jspf"`
	} `json:"payload"`
}

type RadioTrack struct {
	Title      string   `json:"title"`
	Creator    string   `json:"creator"`
	Album      string   `json:"album"`
	Identifier []string `json:"identifier"` // recording MBID URLs
}
