package lastfm

import "fmt"

// APIError represents a Last.fm error response: {"error": 6, "message": "..."}
type APIError struct {
	Code    int    `json:"error"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("lastfm API error %d: %s", e.Code, e.Message)
}

type TopArtistsResponse struct {
	TopArtists TopArtists `json:"topartists"`
}

type TopArtists struct {
	Artist []Artist `json:"artist"`
	Attr   struct {
		User       string `json:"username"`
		TotalPages string `json:"totalPages"`
		Page       string `json:"page"`
		PerPage    string `json:"perPage"`
		Total      string `json:"total"`
	} `json:"@attr"`
}

type SimilarResponse struct {
	SimilarArtists struct {
		Artists []SimilarArtist `json:"artist"`
	} `json:"similarartists"`
}

type TopTagsResponse struct {
	TopTags struct {
		Tag []Tag `json:"tag"`
	} `json:"toptags"`
}

type Artist struct {
	Name      string `json:"name"`
	Playcount string `json:"playcount"`
	Mbid      string `json:"mbid"`
	URL       string `json:"url"`
	Attr      struct {
		Rank string `json:"rank"`
	} `json:"@attr"`
}

type SimilarArtist struct {
	Name  string `json:"name"`
	Match string `json:"match"`
	Mbid  string `json:"mbid"`
	URL   string `json:"url"`
}

type Tag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	URL   string `json:"url"`
}

type Track struct {
	Name      string `json:"name"`
	PlayCount string `json:"playcount"`
	Mbid      string `json:"mbid"`
	URL       string `json:"url"`
	Artist    struct {
		Name string `json:"name"`
		Mbid string `json:"mbid"`
	} `json:"artist"`
}

type TopTracksResponse struct {
	TopTracks struct {
		Tracks []Track `json:"track"`
		Attr   struct {
			User       string `json:"user"`
			TotalPages string `json:"totalPages"`
			Page       string `json:"page"`
			PerPage    string `json:"perPage"`
			Total      string `json:"total"`
		} `json:"@attr"`
	} `json:"toptracks"`
}
