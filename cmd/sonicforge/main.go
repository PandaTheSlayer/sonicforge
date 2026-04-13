package main

import (
	"fmt"
	"log/slog"
	"os"

	"sonicforge/internal/config"
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

			return nil
		},
	}
	cmd.PersistentFlags().StringP("config", "c", "", "config file (default is $HOME/.cobra.yaml)")
	cmd.PersistentFlags().BoolP("verbose", "v", false, "enable debug logging")

	cmd.AddCommand(
		newDiscoverCmd(),
		newProfileCmd(&cfg),
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

func newProfileCmd(cfg **config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Show your taste profile from Last.fm",
		RunE: func(cmd *cobra.Command, args []string) error {
			period, _ := cmd.Flags().GetString("period")
			c := lastfm.NewClient((*cfg).LastFM.APIKey)
			defer c.Close()
			profile, err := c.BuildProfile(cmd.Context(), (*cfg).LastFM.Username, (*cfg).Discovery.TopArtistsCount, period)
			if err != nil {
				return err
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

func newDiscoverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discover",
		Short: "Discovering new music",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("discover: not implemented yet")
			return nil
		},
	}
}
