package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	LastFM struct {
		APIKey   string `mapstructure:"api_key"`
		Username string `mapstructure:"username"`
	} `mapstructure:"lastfm"`
	ListenBrainz struct {
		UserToken string `mapstructure:"user_token"`
		Username  string `mapstructure:"username"`
	} `mapstructure:"listenbrainz"`
	Spotify struct {
		ClientID string `mapstructure:"client_id"`
	} `mapstructure:"spotify"`
	Discovery Discovery `mapstructure:"discovery"`
	Log       struct {
		Level string `mapstructure:"level"`
	} `mapstructure:"log"`
}

type Discovery struct {
	TopArtistsCount     int                           `mapstructure:"top_artists_count"`
	SimilarDepth        int                           `mapstructure:"similar_depth"`
	MaxSimilarPerArtist int                           `mapstructure:"max_similar_per_artist"`
	MinSimilarity       float64                       `mapstructure:"min_similarity"`
	ExcludeListened     bool                          `mapstructure:"exclude_listened"`
	MinListeners        int                           `mapstructure:"min_listeners"`
	PopularityRange     [2]int                        `mapstructure:"popularity_range"`
	PlaylistSize        int                           `mapstructure:"playlist_size"`
	TagsWeight          float64                       `mapstructure:"tags_weight"`
	CFWeight            float64                       `mapstructure:"cf_weight"`
	SimilarityWeight    float64                       `mapstructure:"similarity_weight"`
	ExcludeTags         []string                      `mapstructure:"exclude_tags"`
	BoostTags           []string                      `mapstructure:"boost_tags"`
	PenaltyTags         []string                      `mapstructure:"penalty_tags"`
	ExcludeArtists      []string                      `mapstructure:"exclude_artists"`
	Profiles            map[string]DiscoveryOverrides `mapstructure:"profiles"`
}

type DiscoveryOverrides struct {
	TopArtistsCount     *int     `mapstructure:"top_artists_count"`
	SimilarDepth        *int     `mapstructure:"similar_depth"`
	MaxSimilarPerArtist *int     `mapstructure:"max_similar_per_artist"`
	MinSimilarity       *float64 `mapstructure:"min_similarity"`
	PlaylistSize        *int     `mapstructure:"playlist_size"`
	TagsWeight          *float64 `mapstructure:"tags_weight"`
	CFWeight            *float64 `mapstructure:"cf_weight"`
	SimilarityWeight    *float64 `mapstructure:"similarity_weight"`
	ExcludeTags         []string `mapstructure:"exclude_tags"`
	BoostTags           []string `mapstructure:"boost_tags"`
	PenaltyTags         []string `mapstructure:"penalty_tags"`
	ExcludeArtists      []string `mapstructure:"exclude_artists"`
}

func LoadConfig(path string) (*Config, error) {
	vpr := viper.New()

	if path != "" {
		vpr.SetConfigFile(path)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("getting home directory: %w", err)
		}
		vpr.SetConfigName("config")
		vpr.SetConfigType("yaml")
		vpr.AddConfigPath(".")
		vpr.AddConfigPath(filepath.Join(home, ".config", "sonicforge"))
	}

	if err := vpr.ReadInConfig(); err != nil {
		slog.Debug("config file not found, using defaults/env", "error", err)
	}

	vpr.AutomaticEnv()

	var config Config
	if err := vpr.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return &config, nil
}

func MergeDiscoveryConfig(base Discovery, overrides ...DiscoveryOverrides) Discovery {
	result := base
	for _, override := range overrides {
		if override.TopArtistsCount != nil {
			result.TopArtistsCount = *override.TopArtistsCount
		}
		if override.SimilarDepth != nil {
			result.SimilarDepth = *override.SimilarDepth
		}
		if override.MaxSimilarPerArtist != nil {
			result.MaxSimilarPerArtist = *override.MaxSimilarPerArtist
		}
		if override.MinSimilarity != nil {
			result.MinSimilarity = *override.MinSimilarity
		}
		if override.PlaylistSize != nil {
			result.PlaylistSize = *override.PlaylistSize
		}
		if override.TagsWeight != nil {
			result.TagsWeight = *override.TagsWeight
		}
		if override.CFWeight != nil {
			result.CFWeight = *override.CFWeight
		}
		if override.SimilarityWeight != nil {
			result.SimilarityWeight = *override.SimilarityWeight
		}
		if override.ExcludeTags != nil {
			result.ExcludeTags = override.ExcludeTags
		}
		if override.BoostTags != nil {
			result.BoostTags = override.BoostTags
		}
		if override.PenaltyTags != nil {
			result.PenaltyTags = override.PenaltyTags
		}
		if override.ExcludeArtists != nil {
			result.ExcludeArtists = override.ExcludeArtists
		}
	}
	return result
}

func LoadProfileFile(path string) (DiscoveryOverrides, error) {
	vpr := viper.New()
	vpr.SetConfigFile(path)
	if err := vpr.ReadInConfig(); err != nil {
		return DiscoveryOverrides{}, fmt.Errorf("reading profile file %s: %w", path, err)
	}
	var overrides DiscoveryOverrides
	if err := vpr.Unmarshal(&overrides); err != nil {
		return DiscoveryOverrides{}, fmt.Errorf("unmarshaling profile file: %w", err)
	}
	return overrides, nil
}
