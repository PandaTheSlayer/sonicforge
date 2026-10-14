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
	"sonicforge/internal/listenbrainz"
	"sonicforge/internal/spotify"
	"sonicforge/internal/tui"

	tea "github.com/charmbracelet/bubbletea"

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
		newAuthCmd(&cfg),
		newTuiCmd(&cfg, &appCache),
		newBarCmd(&cfg, &appCache),
	)

	return cmd
}

func newTuiCmd(cfg **config.Config, appCache **cache.Cache) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Interactive discovery TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			period, _ := cmd.Flags().GetString("period")

			discovery, err := resolveDiscoveryConfig(cmd, *cfg)
			if err != nil {
				return err
			}

			c := lastfm.NewClient((*cfg).LastFM.APIKey, *appCache)
			defer c.Close()

			// Build or load profile
			cacheKey := "profile:" + (*cfg).LastFM.Username + ":" + period
			var profile *lastfm.TasteProfile
			if (*appCache).Get(cacheKey, 24*time.Hour, &profile) {
				fmt.Fprintf(os.Stderr, "Using cached profile\n")
			} else {
				fmt.Fprintf(os.Stderr, "Building taste profile...\n")
				profile, err = c.BuildProfile(cmd.Context(), (*cfg).LastFM.Username, discovery.TopArtistsCount, period)
				if err != nil {
					return err
				}
				(*appCache).Set(cacheKey, profile)
			}

			// Generate and rank candidates
			lb := listenbrainz.NewClient((*cfg).ListenBrainz.UserToken, (*cfg).ListenBrainz.Username)
			cfgCopy := **cfg
			cfgCopy.Discovery = discovery
			disc := engine.NewDiscoverer(c, lb, cfgCopy)
			fmt.Fprintf(os.Stderr, "Discovering similar artists...\n")
			candidates, err := disc.GenerateCandidates(cmd.Context(), profile)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Ranking %d candidates...\n", len(candidates))
			ranked := disc.RankAndDedup(cmd.Context(), candidates, profile)

			// Launch TUI
			m := tui.New(ranked)
			p := tea.NewProgram(m, tea.WithAltScreen())
			result, err := p.Run()
			if err != nil {
				return err
			}

			// After TUI exits — create playlist if approved tracks
			finalModel := result.(tui.Model)
			approved := finalModel.Approved()
			if len(approved) == 0 {
				fmt.Println("No tracks approved.")
				return nil
			}

			fmt.Printf("%d tracks approved. Creating Spotify playlist...\n", len(approved))
			sp := spotify.NewClient((*cfg).Spotify.ClientID)
			if err := sp.Authenticate(cmd.Context()); err != nil {
				return err
			}

			var uris []string
			for _, cand := range approved {
				track, err := sp.Search(cmd.Context(), cand.ArtistName, cand.TrackName)
				if err != nil || track == nil {
					continue
				}
				uris = append(uris, track.URI)
			}

			if len(uris) == 0 {
				fmt.Println("No tracks matched on Spotify.")
				return nil
			}

			playlistName := fmt.Sprintf("sonicforge discovery — %s", time.Now().Format("2006-01-02"))
			playlistID, err := sp.CreatePlaylist(cmd.Context(), "", playlistName, "Curated via sonicforge TUI")
			if err != nil {
				return err
			}

			if err := sp.AddTracks(cmd.Context(), playlistID, uris); err != nil {
				return err
			}

			fmt.Printf("Playlist created: %s (%d tracks)\n", playlistName, len(uris))
			return nil
		},
	}
	cmd.Flags().StringP("period", "p", "overall", "time period: overall, 7day, 1month, 3month, 6month, 12month")
	addDiscoveryFlags(cmd)
	return cmd
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

			discovery, err := resolveDiscoveryConfig(cmd, *cfg)
			if err != nil {
				return err
			}

			c := lastfm.NewClient((*cfg).LastFM.APIKey, *appCache)
			defer c.Close()

			cacheKey := "profile:" + (*cfg).LastFM.Username + ":" + period
			var profile *lastfm.TasteProfile
			if (*appCache).Get(cacheKey, 24*time.Hour, &profile) {
				fmt.Fprintf(os.Stderr, "Using cached profile\n")
			} else {
				fmt.Fprintf(os.Stderr, "Building taste profile...\n")
				profile, err = c.BuildProfile(cmd.Context(), (*cfg).LastFM.Username, discovery.TopArtistsCount, period)
				if err != nil {
					return err
				}
				(*appCache).Set(cacheKey, profile)
			}

			lb := listenbrainz.NewClient((*cfg).ListenBrainz.UserToken, (*cfg).ListenBrainz.Username)
			cfgCopy := **cfg
			cfgCopy.Discovery = discovery
			disc := engine.NewDiscoverer(c, lb, cfgCopy)
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

			createPlaylist, _ := cmd.Flags().GetBool("playlist")
			if !createPlaylist {
				return nil
			}

			// Spotify: auth, search, create playlist
			sp := spotify.NewClient((*cfg).Spotify.ClientID)
			if err := sp.Authenticate(cmd.Context()); err != nil {
				return fmt.Errorf("spotify auth: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Matching tracks on Spotify...\n")
			var uris []string
			for i, cand := range ranked {
				fmt.Fprintf(os.Stderr, "\rSearching... %d/%d", i+1, len(ranked))
				track, err := sp.Search(cmd.Context(), cand.ArtistName, cand.TrackName)
				if err != nil {
					slog.Debug("spotify search failed", "artist", cand.ArtistName, "error", err)
					continue
				}
				if track == nil {
					slog.Debug("no spotify match", "artist", cand.ArtistName, "track", cand.TrackName)
					continue
				}
				uris = append(uris, track.URI)
			}
			fmt.Fprintf(os.Stderr, "\rMatched %d/%d tracks on Spotify\n", len(uris), len(ranked))

			if len(uris) == 0 {
				fmt.Println("No tracks matched on Spotify.")
				return nil
			}

			userID, err := sp.GetCurrentUser(cmd.Context())
			if err != nil {
				return fmt.Errorf("getting spotify user: %w", err)
			}

			playlistName := fmt.Sprintf("sonicforge discovery — %s", time.Now().Format("2006-01-02"))
			playlistID, err := sp.CreatePlaylist(cmd.Context(), userID, playlistName, "Auto-generated from Last.fm")
			if err != nil {
				return fmt.Errorf("creating playlist: %w", err)
			}

			if err := sp.AddTracks(cmd.Context(), playlistID, uris); err != nil {
				return fmt.Errorf("adding tracks: %w", err)
			}

			fmt.Printf("\nPlaylist created: %s (%d tracks)\n", playlistName, len(uris))

			return nil
		},
	}
	cmd.Flags().StringP("period", "p", "overall", "time period: overall, 7day, 1month, 3month, 6month, 12month")
	cmd.Flags().Bool("playlist", false, "create Spotify playlist from results")
	addDiscoveryFlags(cmd)
	return cmd
}

func newAuthCmd(cfg **config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with Spotify",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := spotify.NewClient((*cfg).Spotify.ClientID)
			fmt.Fprintf(os.Stderr, "Starting Spotify authentication...\n")
			if err := c.Login(cmd.Context()); err != nil {
				return err
			}
			fmt.Println("Authenticated successfully!")
			return nil
		},
	}
}

func addDiscoveryFlags(cmd *cobra.Command) {
	cmd.Flags().String("profile", "", "named profile from config.yaml (discovery.profiles.<name>)")
	cmd.Flags().String("profile-file", "", "path to external profile YAML")
	cmd.Flags().Int("playlist-size", 0, "override playlist size")
	cmd.Flags().StringSlice("boost-tags", nil, "comma-separated boost tags")
	cmd.Flags().StringSlice("penalty-tags", nil, "comma-separated penalty tags")
	cmd.Flags().StringSlice("exclude-tags", nil, "comma-separated exclude tags")
	cmd.Flags().StringSlice("exclude-artists", nil, "comma-separated exclude artists")
	cmd.Flags().Float64("tags-weight", 0, "override tags weight")
	cmd.Flags().Float64("similarity-weight", 0, "override similarity weight")
}

func resolveDiscoveryConfig(cmd *cobra.Command, cfg *config.Config) (config.Discovery, error) {
	var overrides []config.DiscoveryOverrides

	if name, _ := cmd.Flags().GetString("profile"); name != "" {
		ov, ok := cfg.Discovery.Profiles[name]
		if !ok {
			return config.Discovery{}, fmt.Errorf("profile %q not found in config (available: %v)", name, profileNames(cfg.Discovery.Profiles))
		}
		slog.Debug("applying named profile", "name", name)
		overrides = append(overrides, ov)
	}

	if path, _ := cmd.Flags().GetString("profile-file"); path != "" {
		ov, err := config.LoadProfileFile(path)
		if err != nil {
			return config.Discovery{}, err
		}
		slog.Debug("applying profile file", "path", path)
		overrides = append(overrides, ov)
	}

	overrides = append(overrides, buildOverridesFromFlags(cmd))

	return config.MergeDiscoveryConfig(cfg.Discovery, overrides...), nil
}

func profileNames(m map[string]config.DiscoveryOverrides) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	return names
}

func buildOverridesFromFlags(cmd *cobra.Command) config.DiscoveryOverrides {
	var ov config.DiscoveryOverrides
	f := cmd.Flags()

	if f.Changed("playlist-size") {
		v, _ := f.GetInt("playlist-size")
		ov.PlaylistSize = &v
	}
	if f.Changed("tags-weight") {
		v, _ := f.GetFloat64("tags-weight")
		ov.TagsWeight = &v
	}
	if f.Changed("similarity-weight") {
		v, _ := f.GetFloat64("similarity-weight")
		ov.SimilarityWeight = &v
	}
	if f.Changed("boost-tags") {
		v, _ := f.GetStringSlice("boost-tags")
		ov.BoostTags = v
	}
	if f.Changed("penalty-tags") {
		v, _ := f.GetStringSlice("penalty-tags")
		ov.PenaltyTags = v
	}
	if f.Changed("exclude-tags") {
		v, _ := f.GetStringSlice("exclude-tags")
		ov.ExcludeTags = v
	}
	if f.Changed("exclude-artists") {
		v, _ := f.GetStringSlice("exclude-artists")
		ov.ExcludeArtists = v
	}

	return ov
}
