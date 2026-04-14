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
	Discovery struct {
		TopArtistsCount     int      `mapstructure:"top_artists_count"`
		SimilarDepth        int      `mapstructure:"similar_depth"`
		MaxSimilarPerArtist int      `mapstructure:"max_similar_per_artist"`
		MinSimilarity       float64  `mapstructure:"min_similarity"`
		ExcludeListened     bool     `mapstructure:"exclude_listened"`
		MinListeners        int      `mapstructure:"min_listeners"`
		PopularityRange     [2]int   `mapstructure:"popularity_range"`
		PlaylistSize        int      `mapstructure:"playlist_size"`
		TagsWeight          float64  `mapstructure:"tags_weight"`
		CFWeight            float64  `mapstructure:"cf_weight"`
		SimilarityWeight    float64  `mapstructure:"similarity_weight"`
		ExcludeTags         []string `mapstructure:"exclude_tags"`
		BoostTags           []string `mapstructure:"boost_tags"`
		PenaltyTags         []string `mapstructure:"penalty_tags"`
		ExcludeArtists      []string `mapstructure:"exclude_artists"`
	} `mapstructure:"discovery"`
	Log struct {
		Level string `mapstructure:"level"`
	} `mapstructure:"log"`
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
