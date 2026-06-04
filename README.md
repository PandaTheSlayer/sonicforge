# sonicforge

A music-discovery CLI + TUI that builds Spotify playlists from your **Last.fm**
listening history, **ListenBrainz** collaborative filtering, and **MusicBrainz**
metadata — without relying on Spotify's own recommendation endpoints.

## Why

Spotify's recommendation algorithm tends to loop the same tracks, and as of
November 2024 its Audio Features, Recommendations, and Related Artists endpoints
are deprecated for Dev Mode apps. If you have years of Last.fm scrobbles and a
ListenBrainz account, those are richer, more personal discovery signals.

sonicforge uses Last.fm + ListenBrainz to *decide what to recommend*, and uses
Spotify purely as **output** (search + playlist creation).

## How it works

```
Last.fm history ─┐
ListenBrainz CF ─┼─▶ Taste profile ─▶ Discovery engine ─▶ Spotify
MusicBrainz IDs ─┘   (top artists,     (2-hop similar      (search +
                      tags, known       graph, tag-based    create
                      tracks)           ranking, dedup)     playlist)
```

1. **Taste profile** — top artists / tracks / genre tags from Last.fm.
2. **Candidate generation** — a 2-hop "similar artists" graph plus ListenBrainz
   tag-radio (and, in progress, collaborative-filtering recommendations).
3. **Ranking & dedup** — tag-overlap scoring with boost/penalty weighting, then
   deduplication against what you've already heard.
4. **Curation** — review candidates in an interactive TUI (approve / skip).
5. **Output** — matched to Spotify and written to a private playlist.

## Status

The core pipeline works end-to-end: Last.fm client, discovery engine, Spotify
OAuth + playlist creation, ListenBrainz tag-radio, file-based caching, and an
interactive TUI. A track-seeded "radio" mode and ListenBrainz CF wiring are in
progress.

See [`docs/ROADMAP.md`](docs/ROADMAP.md) for the detailed, per-feature status and
the active milestones (M1–M3).

## Install

Requires a Go toolchain matching `go.mod` (currently `go 1.26`).

```bash
git clone <repo-url> sonicforge
cd sonicforge
go build -o sonicforge ./cmd/sonicforge
./sonicforge --help
```

## Configuration

sonicforge looks for `config.yaml` in the current directory or in
`~/.config/sonicforge/`. You can also point at a file explicitly with
`--config <path>`.

```yaml
lastfm:
  api_key: "your-lastfm-api-key"     # https://www.last.fm/api/account/create
  username: "your-lastfm-username"

listenbrainz:
  user_token: "your-lb-token"        # https://listenbrainz.org/settings/
  username: "your-lb-username"

spotify:
  client_id: "your-spotify-client-id"   # PKCE flow — no client secret needed

discovery:
  top_artists_count: 50
  similar_depth: 2
  max_similar_per_artist: 10
  min_similarity: 0.3
  exclude_listened: true
  min_listeners: 1000
  popularity_range: [0, 80]
  playlist_size: 30
  tags_weight: 0.4
  cf_weight: 0.3
  similarity_weight: 0.3
  boost_tags: []                     # tags that multiply a candidate's score up
  penalty_tags: []                   # tags that multiply it down
  exclude_tags: []
  exclude_artists: []

  # Optional named presets, applied with --profile <name>
  profiles:
    doom:
      playlist_size: 40
      boost_tags: ["doom metal", "sludge"]
```

> **Spotify Dev Mode** requires the app owner to have Spotify Premium. sonicforge
> uses the Authorization Code + PKCE flow; tokens are cached under
> `~/.config/sonicforge/`. Required scopes:
> `playlist-modify-public playlist-modify-private user-top-read user-library-read`.

## Usage

```bash
# Show your Last.fm taste profile (top artists + tags)
sonicforge profile

# Authenticate with Spotify (opens a browser; one-time)
sonicforge auth

# Discover candidates and print them
sonicforge discover

# Discover and create a Spotify playlist
sonicforge discover --playlist

# Interactive curation, then build a playlist from approved tracks
sonicforge tui

# Inspect / debug the signals behind a recommendation
sonicforge inspect artist-tags "Sleep"
sonicforge inspect artist-similar "Sleep"
sonicforge inspect track-tags "Sleep" "Dopesmoker"
sonicforge inspect track-similar "Sleep" "Dopesmoker"
sonicforge inspect recent
```

### Common flags

Global:

| Flag | Description |
|------|-------------|
| `-c, --config <path>` | Use a specific config file |
| `-v, --verbose` | Debug logging |

`discover` / `tui`:

| Flag | Description |
|------|-------------|
| `-p, --period <p>` | Last.fm window: `overall`, `7day`, `1month`, `3month`, `6month`, `12month` |
| `--playlist` | (`discover` only) create a Spotify playlist from results |
| `--profile <name>` | Apply a named preset from `discovery.profiles` |
| `--profile-file <path>` | Apply an external profile YAML |
| `--playlist-size <n>` | Override target playlist size |
| `--boost-tags`, `--penalty-tags`, `--exclude-tags` | Comma-separated tag tuning |
| `--exclude-artists` | Comma-separated artists to drop |
| `--tags-weight`, `--similarity-weight` | Override ranking weights |

Tuning precedence: base `discovery` config → `--profile` → `--profile-file` →
individual flags (later wins).

## Rate limits

sonicforge respects each service's limits: Last.fm 5 req/sec, MusicBrainz 1
req/sec, ListenBrainz header-based, Spotify with back-off on 429. Last.fm
responses (similar artists, tags) are cached on disk under `~/.cache/sonicforge`,
so repeat runs are fast; the first run for a new profile can be slow.

## Project layout

```
cmd/sonicforge/   CLI entry point + commands (discover, profile, tui, auth, inspect)
internal/
  config/         YAML config + named profiles + flag overrides
  lastfm/         Last.fm client, models, taste profile
  listenbrainz/   ListenBrainz client, CF + tag-radio
  musicbrainz/    MBID → artist/track resolution (planned)
  spotify/        OAuth2 PKCE auth, search, playlist creation
  engine/         Discovery pipeline: graph, ranker, dedup, radio
  tui/            bubbletea interactive curation UI
  cache/          File-based JSON cache with TTLs
docs/             ARCHITECTURE.md, API_REFERENCE.md, ROADMAP.md
```

## Notes

This is a personal project. Code is written by hand; it favors simple, idiomatic
Go and the standard library over heavy frameworks. Built with
[bubbletea](https://github.com/charmbracelet/bubbletea),
[lipgloss](https://github.com/charmbracelet/lipgloss),
[cobra](https://github.com/spf13/cobra), and
[viper](https://github.com/spf13/viper).
