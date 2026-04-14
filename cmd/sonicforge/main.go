package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"sonicforge/internal/cache"
	"sonicforge/internal/config"
	"sonicforge/internal/engine"
	"sonicforge/internal/lastfm"

	"github.com/spf13/cobra"
)

func main() {
	cmd := newRootCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var cfg *config.Config
	var appCache *cache.Cache
	cmd := &cobra.Command{
		Use:   "sonicforge",
		Short: "Music discovery from Last.fm + ListenBrainz -> Spotify",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			level := slog.LevelInfo
			if verbose {
				level = slog.LevelDebug
			}

			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: level,
			}))
			slog.SetDefault(logger)

			path, _ := cmd.Flags().GetString("config")
			var err error
			cfg, err = config.LoadConfig(path)
			if err != nil {
				return err
			}

			slog.Debug("config loaded", "lastfm_user", cfg.LastFM.Username)

			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			appCache, err = cache.New(filepath.Join(home, ".cache", "sonicforge"))
			if err != nil {
				return err
			}

			return nil
		},
	}
	cmd.PersistentFlags().StringP("config", "c", "", "config file (default is $HOME/.cobra.yaml)")
	cmd.PersistentFlags().BoolP("verbose", "v", false, "enable debug logging")

	cmd.AddCommand(
		newDiscoverCmd(&cfg, &appCache),
		newProfileCmd(&cfg, &appCache),
		newTuiCmd(),
	)

	return cmd
}

func newTuiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Enter sonicforge TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("tui: not implemented yet")
			return nil
		},
	}
}

func newProfileCmd(cfg **config.Config, appCache **cache.Cache) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Show your taste profile from Last.fm",
		RunE: func(cmd *cobra.Command, args []string) error {
			period, _ := cmd.Flags().GetString("period")
			cacheKey := "profile:" + (*cfg).LastFM.Username + ":" + period

			var profile *lastfm.TasteProfile
			if (*appCache).Get(cacheKey, 24*time.Hour, &profile) {
				fmt.Fprintf(os.Stderr, "Using cached profile\n")
			} else {
				c := lastfm.NewClient((*cfg).LastFM.APIKey, *appCache)
				defer c.Close()
				var err error
				profile, err = c.BuildProfile(cmd.Context(), (*cfg).LastFM.Username, (*cfg).Discovery.TopArtistsCount, period)
				if err != nil {
					return err
				}
				(*appCache).Set(cacheKey, profile)
			}

			fmt.Printf("Top Artists (%d):\n", len(profile.TopArtists))
			for i, a := range profile.TopArtists {
				if i >= 10 {
					break
				}
				fmt.Printf("  %d. %s (%d plays)\n", i+1, a.Name, a.Playcount)
			}

			fmt.Printf("\nTop Tags (%d):\n", len(profile.TopTags))
			for i, t := range profile.TopTags {
				if i >= 10 {
					break
				}
				fmt.Printf("  %d. %s (weight: %.0f)\n", i+1, t.Tag, t.CumulativeWeight)
			}

			fmt.Printf("\nKnown: %d artists, %d tracks\n",
				len(profile.KnownArtists), len(profile.KnownTracks))

			return nil
		},
	}
	cmd.Flags().StringP("period", "p", "overall", "time period: overall, 7day, 1month, 3month, 6month, 12month")
	return cmd
}

func newDiscoverCmd(cfg **config.Config, appCache **cache.Cache) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover new music based on your taste profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			period, _ := cmd.Flags().GetString("period")
			c := lastfm.NewClient((*cfg).LastFM.APIKey, *appCache)
			defer c.Close()

			cacheKey := "profile:" + (*cfg).LastFM.Username + ":" + period
			var profile *lastfm.TasteProfile
			if (*appCache).Get(cacheKey, 24*time.Hour, &profile) {
				fmt.Fprintf(os.Stderr, "Using cached profile\n")
			} else {
				fmt.Fprintf(os.Stderr, "Building taste profile...\n")
				var err error
				profile, err = c.BuildProfile(cmd.Context(), (*cfg).LastFM.Username, (*cfg).Discovery.TopArtistsCount, period)
				if err != nil {
					return err
				}
				(*appCache).Set(cacheKey, profile)
			}

			disc := engine.NewDiscoverer(c, **cfg)
			fmt.Fprintf(os.Stderr, "Discovering similar artists...\n")
			candidates, err := disc.GenerateCandidates(cmd.Context(), profile)
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Ranking %d candidates...\n", len(candidates))
			ranked := disc.RankAndDedup(cmd.Context(), candidates, profile)

			fmt.Printf("Top candidates (%d):\n", len(ranked))
			for i, cand := range ranked {
				fmt.Printf("  %d. %s (score: %.3f, similarity: %.2f, tag overlap: %.2f, source: %s)\n",
					i+1, cand.ArtistName, cand.CompositeScore, cand.SimilarityScore, cand.TagOverlapScore, cand.Source)
			}

			return nil
		},
	}
	cmd.Flags().StringP("period", "p", "overall", "time period: overall, 7day, 1month, 3month, 6month, 12month")
	return cmd
}
