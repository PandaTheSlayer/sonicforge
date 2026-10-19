package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"sonicforge/internal/cache"
	"sonicforge/internal/config"
	"sonicforge/internal/lastfm"
	"sonicforge/internal/spotify"

	"github.com/spf13/cobra"
)

type barTrack struct {
	ID         string   `json:"id"`
	URI        string   `json:"uri"`
	Name       string   `json:"name"`
	Artists    []string `json:"artists"`
	Album      string   `json:"album"`
	DurationMs int      `json:"durationMs"`
	Image      string   `json:"image"`
}

func toBarTrack(t *spotify.PlayerTrack, imageSize int) *barTrack {
	if t == nil {
		return nil
	}
	artists := make([]string, 0, len(t.Artists))
	for _, a := range t.Artists {
		artists = append(artists, a.Name)
	}
	return &barTrack{
		ID:         t.ID,
		URI:        t.URI,
		Name:       t.Name,
		Artists:    artists,
		Album:      t.Album.Name,
		DurationMs: t.DurationMs,
		Image:      spotify.SmallestImage(t.Album.Images, imageSize),
	}
}

func printJSON(v any) {
	data, _ := json.Marshal(v)
	fmt.Println(string(data))
}

func barRun(cfg **config.Config, fn func(ctx context.Context, sp *spotify.Client) (any, error)) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		sp := spotify.NewClient((*cfg).Spotify.ClientID)
		var result any
		err := sp.AuthenticateStored(ctx)
		if err == nil {
			result, err = fn(ctx, sp)
		}
		if err != nil {
			msg := err.Error()
			if errors.Is(err, spotify.ErrNotAuthenticated) {
				msg = "auth"
			} else if errors.Is(err, spotify.ErrNoDevice) {
				msg = "no-device"
			}
			printJSON(map[string]string{"error": msg})
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			os.Exit(1)
		}
		printJSON(result)
		return nil
	}
}

func newBarCmd(cfg **config.Config, appCache **cache.Cache) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bar",
		Short: "JSON interface for the Zebar panel: now, like, unlike, queue, radio, queue-play, volume",
	}

	now := &cobra.Command{
		Use:   "now",
		Short: "Current Spotify track and whether it is in Liked Songs",
		RunE: barRun(cfg, func(ctx context.Context, sp *spotify.Client) (any, error) {
			cp, err := sp.CurrentlyPlaying(ctx)
			if err != nil {
				return nil, err
			}
			if cp == nil || cp.Item == nil {
				return map[string]any{"track": nil}, nil
			}
			liked := false
			if res, err := sp.InLibrary(ctx, []string{cp.Item.URI}); err == nil && len(res) == 1 {
				liked = res[0]
			} else if err != nil {
				slog.Debug("library check failed", "error", err)
			}
			return map[string]any{
				"track":      toBarTrack(cp.Item, 300),
				"isPlaying":  cp.IsPlaying,
				"progressMs": cp.ProgressMs,
				"liked":      liked,
			}, nil
		}),
	}

	likeCmd := func(use string, liked bool) *cobra.Command {
		return &cobra.Command{
			Use:   use + " <spotify:track:uri>",
			Short: "Save or remove a track from Liked Songs",
			Args:  cobra.ExactArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				uri := args[0]
				return barRun(cfg, func(ctx context.Context, sp *spotify.Client) (any, error) {
					var err error
					if liked {
						err = sp.SaveToLibrary(ctx, []string{uri})
					} else {
						err = sp.RemoveFromLibrary(ctx, []string{uri})
					}
					if err != nil {
						return nil, err
					}
					return map[string]any{"uri": uri, "liked": liked}, nil
				})(c, args)
			},
		}
	}

	queue := &cobra.Command{
		Use:   "queue",
		Short: "Next five tracks in the playback queue",
		RunE: barRun(cfg, func(ctx context.Context, sp *spotify.Client) (any, error) {
			tracks, err := sp.Queue(ctx, 5)
			if err != nil {
				return nil, err
			}
			items := make([]*barTrack, 0, len(tracks))
			for i := range tracks {
				items = append(items, toBarTrack(&tracks[i], 64))
			}
			return map[string]any{"items": items}, nil
		}),
	}

	var radioSeed string
	var radioSize int
	radio := &cobra.Command{
		Use:   "radio",
		Short: "Tracks like the seed (Last.fm similar artists, matched on Spotify); creates nothing",
		RunE: barRun(cfg, func(ctx context.Context, sp *spotify.Client) (any, error) {
			seed, err := seedTrack(ctx, sp, radioSeed)
			if err != nil {
				return nil, err
			}
			lfm := lastfm.NewClient((*cfg).LastFM.APIKey, *appCache)
			defer lfm.Close()
			items, err := buildRadio(ctx, lfm, sp, seed, radioSize)
			if err != nil {
				return nil, err
			}
			return map[string]any{"seed": seed.URI, "items": items}, nil
		}),
	}
	radio.Flags().StringVar(&radioSeed, "seed", "", "spotify:track URI to build from (default: the current track)")
	radio.Flags().IntVar(&radioSize, "size", 25, "number of tracks")
	radio.Flags().Bool("json", true, "print JSON (always on for bar)")

	queuePlay := &cobra.Command{
		Use:   "queue-play <spotify:track:uri>",
		Short: "Play the track now through the queue; the current album or playlist resumes after it",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			uri := args[0]
			return barRun(cfg, func(ctx context.Context, sp *spotify.Client) (any, error) {
				return queuePlay(ctx, sp, uri)
			})(c, args)
		},
	}
	queuePlay.Flags().Bool("json", true, "print JSON (always on for bar)")

	volume := &cobra.Command{
		Use:   "volume [0-100]",
		Short: "Spotify app volume of the active device; with a number, set it first",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			set := -1
			if len(args) == 1 {
				n, err := strconv.Atoi(args[0])
				if err != nil || n < 0 || n > 100 {
					return fmt.Errorf("volume must be 0-100, got %q", args[0])
				}
				set = n
			}
			return barRun(cfg, func(ctx context.Context, sp *spotify.Client) (any, error) {
				if set >= 0 {
					if err := sp.SetVolume(ctx, set); err != nil {
						return nil, err
					}
					// the player reports the new level with a delay; echo what was set
					return map[string]any{"volume": set, "supports": true}, nil
				}
				st, err := sp.Playback(ctx)
				if err != nil {
					return nil, err
				}
				if st == nil || st.Device == nil {
					return nil, spotify.ErrNoDevice
				}
				res := map[string]any{"supports": st.Device.SupportsVolume, "device": st.Device.Name}
				if st.Device.VolumePercent != nil {
					res["volume"] = *st.Device.VolumePercent
				}
				return res, nil
			})(c, args)
		},
	}

	cmd.AddCommand(now, likeCmd("like", true), likeCmd("unlike", false), queue, radio, queuePlay, volume)
	return cmd
}

func queuePlay(ctx context.Context, sp *spotify.Client, uri string) (any, error) {
	if err := sp.AddToQueue(ctx, uri); err != nil {
		return nil, err
	}
	playing := ""
	for i := 0; i < 11; i++ {
		time.Sleep(400 * time.Millisecond)
		if err := sp.Next(ctx); err != nil {
			return nil, err
		}
		time.Sleep(500 * time.Millisecond)
		cp, err := sp.CurrentlyPlaying(ctx)
		if err != nil {
			return nil, err
		}
		if cp != nil && cp.Item != nil {
			playing = cp.Item.URI
		}
		if playing == uri {
			break
		}
	}
	return map[string]any{"playing": playing, "ok": playing == uri}, nil
}

type radioItem struct {
	URI        string `json:"uri"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	DurationMs int    `json:"duration_ms"`
	Cover      string `json:"cover"`
}

func seedTrack(ctx context.Context, sp *spotify.Client, uri string) (*spotify.PlayerTrack, error) {
	cp, err := sp.CurrentlyPlaying(ctx)
	if err == nil && cp != nil && cp.Item != nil && (uri == "" || cp.Item.URI == uri) {
		return cp.Item, nil
	}
	if uri == "" {
		return nil, errors.New("nothing is playing")
	}
	return sp.Track(ctx, strings.TrimPrefix(uri, "spotify:track:"))
}

func buildRadio(ctx context.Context, lfm *lastfm.Client, sp *spotify.Client, seed *spotify.PlayerTrack, size int) ([]radioItem, error) {
	if len(seed.Artists) == 0 {
		return nil, errors.New("seed track has no artist")
	}
	artist := seed.Artists[0].Name
	similar, err := lfm.GetSimilar(ctx, artist, 20)
	if err != nil {
		return nil, fmt.Errorf("lastfm similar: %w", err)
	}

	type pick struct{ artist, track string }
	var picks []pick
	seen := map[string]bool{strings.ToLower(artist + "\x00" + seed.Name): true}
	add := func(a, t string) {
		key := strings.ToLower(a + "\x00" + t)
		if !seen[key] {
			seen[key] = true
			picks = append(picks, pick{a, t})
		}
	}

	for _, s := range similar {
		if len(picks) >= size*2 {
			break
		}
		tops, err := lfm.GetArtistTopTracks(ctx, s.Name, 5)
		if err != nil {
			slog.Debug("lastfm top tracks failed", "artist", s.Name, "error", err)
			continue
		}
		rand.Shuffle(len(tops), func(i, j int) { tops[i], tops[j] = tops[j], tops[i] })
		for i := 0; i < len(tops) && i < 2; i++ {
			add(s.Name, tops[i].Name)
		}
	}
	if tops, err := lfm.GetArtistTopTracks(ctx, artist, 4); err == nil {
		for _, t := range tops {
			add(artist, t.Name)
		}
	}
	rand.Shuffle(len(picks), func(i, j int) { picks[i], picks[j] = picks[j], picks[i] })

	items := []radioItem{}
	for _, p := range picks {
		if len(items) >= size {
			break
		}
		found, err := sp.Search(ctx, p.artist, p.track)
		if err != nil || found == nil {
			slog.Debug("no spotify match", "artist", p.artist, "track", p.track, "error", err)
			continue
		}
		name := p.artist
		if len(found.Artists) > 0 {
			name = found.Artists[0].Name
		}
		items = append(items, radioItem{
			URI:        found.URI,
			Title:      found.Name,
			Artist:     name,
			DurationMs: found.DurationMs,
			Cover:      spotify.SmallestImage(found.Album.Images, 64),
		})
	}

	return items, nil
}
