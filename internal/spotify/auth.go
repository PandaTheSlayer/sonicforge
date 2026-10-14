package spotify

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	authURL     = "https://accounts.spotify.com/authorize"
	tokenURL    = "https://accounts.spotify.com/api/token"
	redirectURI = "http://127.0.0.1:8888/callback"
	scopes      = "playlist-modify-public playlist-modify-private user-top-read user-library-read " +
		"user-library-modify user-read-playback-state user-read-currently-playing user-modify-playback-state"
)

var ErrNotAuthenticated = errors.New("spotify: not authenticated, run `sonicforge auth`")

type storedTokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Authenticate ensures we have valid tokens.
// Loads from disk, refreshes if expired, or starts PKCE flow if no tokens.
func (c *Client) Authenticate(ctx context.Context) error {
	if err := c.AuthenticateStored(ctx); err == nil {
		return nil
	} else {
		slog.Debug("stored tokens unusable, starting new auth", "error", err)
	}
	return c.Login(ctx)
}

func (c *Client) AuthenticateStored(ctx context.Context) error {
	tokens, err := c.loadTokens()
	if err != nil {
		return ErrNotAuthenticated
	}
	if time.Now().Before(tokens.ExpiresAt) {
		slog.Debug("using stored spotify tokens")
		c.accessToken = tokens.AccessToken
		return nil
	}
	if tokens.RefreshToken == "" {
		return ErrNotAuthenticated
	}

	slog.Debug("refreshing spotify tokens")
	resp, err := c.refreshTokens(ctx, tokens.RefreshToken)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotAuthenticated, err)
	}
	st := &storedTokens{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second),
	}
	if st.RefreshToken == "" {
		st.RefreshToken = tokens.RefreshToken
	}
	c.accessToken = resp.AccessToken
	return c.saveTokens(st)
}

func (c *Client) Login(ctx context.Context) error {
	slog.Debug("starting PKCE flow")
	resp, err := c.pkceFlow(ctx)
	if err != nil {
		return err
	}

	st := &storedTokens{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second),
	}
	c.accessToken = resp.AccessToken
	return c.saveTokens(st)
}

func (c *Client) tokensPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "sonicforge", "tokens.json")
}

func (c *Client) loadTokens() (*storedTokens, error) {
	data, err := os.ReadFile(c.tokensPath())
	if err != nil {
		return nil, err
	}
	var tokens storedTokens
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, err
	}
	return &tokens, nil
}

func (c *Client) saveTokens(tokens *storedTokens) error {
	if err := os.MkdirAll(filepath.Dir(c.tokensPath()), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.tokensPath(), data, 0o600)
}

func (c *Client) refreshTokens(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", c.clientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("spotify refresh: no access token in response")
	}

	if tokenResp.RefreshToken == "" {
		tokenResp.RefreshToken = refreshToken
	}

	return &tokenResp, nil
}

func (c *Client) exchangeCode(ctx context.Context, code, codeVerifier string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", c.clientID)
	data.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("spotify auth: no access token in response")
	}

	return &tokenResp, nil
}

func (c *Client) pkceFlow(ctx context.Context) (*TokenResponse, error) {
	// TODO:
	// 1. Generate code_verifier (random 64 bytes, base64url)
	codeVerifier, err := generateVerifier()
	if err != nil {
		return nil, err
	}
	// 2. Generate code_challenge = base64url(sha256(verifier))
	codeChallenge := generateChallenge(codeVerifier)
	// 3. Open browser to authURL with challenge
	params := url.Values{}
	params.Set("client_id", c.clientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", scopes)
	params.Set("code_challenge_method", "S256")
	params.Set("code_challenge", codeChallenge)
	authLink := authURL + "?" + params.Encode()

	// Open in browser
	fmt.Fprintf(os.Stderr, "Opening browser for Spotify login...\n")
	fmt.Fprintf(os.Stderr, "If it does not open, visit:\n%s\n", authLink)
	openBrowser(authLink)
	// 4. Start local HTTP server on :8888 to receive callback
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	srv := &http.Server{Addr: ":8888", Handler: mux}
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no code in callback: %s", r.URL.Query().Get("error"))
			fmt.Fprintf(w, "Authentication failed. Check terminal.")
			return
		}
		fmt.Fprintf(w, "Success! You can close this tab.")
		codeCh <- code
	})

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return nil, err
	}

	srv.Shutdown(ctx)

	// 5. Exchange auth code for tokens
	return c.exchangeCode(ctx, code, codeVerifier)
}

// generateVerifier creates a random PKCE code verifier
func generateVerifier() (string, error) {
	bytes := make([]byte, 64)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// generateChallenge creates S256 code challenge from verifier
func generateChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func openBrowser(link string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", link)
	case "darwin":
		cmd = exec.Command("open", link)
	default:
		cmd = exec.Command("xdg-open", link)
	}
	if err := cmd.Start(); err != nil {
		slog.Debug("could not open browser", "error", err)
	}
}
