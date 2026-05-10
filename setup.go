package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const spotifyRedirectURI = "http://127.0.0.1:8888/callback"
const spotifyScopes = "user-read-currently-playing user-read-playback-state"

func runSetup() error {
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║   spotslack — OAuth Setup Wizard     ║")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()

	cfg := &Config{
		Emojis: []string{"🎧", "🎵", "🎶"},
	}

	// ── Step 1: Spotify ──────────────────────────────────────────────────────
	fmt.Println("STEP 1: Spotify")
	fmt.Println("  → Go to https://developer.spotify.com/dashboard")
	fmt.Println("  → Create an app (any name)")
	fmt.Printf("  → Add redirect URI: %s\n", spotifyRedirectURI)
	fmt.Println()

	cfg.SpotifyClientID = prompt("  Paste your Spotify Client ID: ")
	cfg.SpotifyClientSecret = prompt("  Paste your Spotify Client Secret: ")
	fmt.Println()

	if err := spotifyOAuthFlow(cfg); err != nil {
		return fmt.Errorf("spotify OAuth: %w", err)
	}
	fmt.Println("  ✅ Spotify connected!")
	fmt.Println()

	// ── Step 2: Slack ─────────────────────────────────────────────────────────
	fmt.Println("STEP 2: Slack")
	fmt.Println("  → Go to https://api.slack.com/apps → Create New App → From scratch")
	fmt.Println("  → Under 'OAuth & Permissions', add these User Token Scopes:")
	fmt.Println("      users.profile:write")
	fmt.Println("      users.profile:read")
	fmt.Println("      team:read")
	fmt.Println("  → Click 'Install App to Workspace'")
	fmt.Println("  → Copy the 'User OAuth Token' (starts with xoxp-)")
	fmt.Println()

	cfg.SlackToken = prompt("  Paste your Slack User OAuth Token: ")

	// Validate and get team name
	fmt.Print("  Validating Slack token... ")
	teamName, err := getSlackTeamName(cfg.SlackToken)
	if err != nil {
		fmt.Println("❌")
		return fmt.Errorf("invalid Slack token: %w", err)
	}
	cfg.SlackTeam = teamName
	fmt.Printf("✅ Connected to '%s'\n\n", teamName)

	// ── Step 3: Save ─────────────────────────────────────────────────────────
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	if err := saveConfigToFile(cfg, path); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	fmt.Printf("✅ Config saved to %s\n\n", path)

	// ── Step 4: Test ─────────────────────────────────────────────────────────
	fmt.Println("Testing connection...")
	if err := cfg.RefreshSpotifyTokenIfNeeded(); err == nil {
		track, playing, err := getCurrentTrack(cfg.SpotifyAccessToken)
		if err == nil {
			if playing {
				fmt.Printf("🎵 Now playing: %s — %s\n", track.Name, track.Artist)
			} else {
				fmt.Println("⏹️  Spotify: nothing currently playing")
			}
		}
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║          Setup complete! 🎉           ║")
	fmt.Println("╠══════════════════════════════════════╣")
	fmt.Println("║  Run the daemon:                     ║")
	fmt.Println("║    spotslack                         ║")
	fmt.Println("║                                      ║")
	fmt.Println("║  Auto-start with launchd:            ║")
	fmt.Println("║    make install                      ║")
	fmt.Println("╚══════════════════════════════════════╝")

	return nil
}

func spotifyOAuthFlow(cfg *Config) error {
	// Build auth URL
	params := url.Values{}
	params.Set("client_id", cfg.SpotifyClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", spotifyRedirectURI)
	params.Set("scope", spotifyScopes)
	authURL := "https://accounts.spotify.com/authorize?" + params.Encode()

	fmt.Println("  Opening Spotify authorization in browser...")
	fmt.Printf("  If it doesn't open, visit:\n  %s\n\n", authURL)
	openBrowser(authURL)

	// Local HTTP server to catch callback
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	srv := &http.Server{Addr: ":8888"}
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		errParam := r.URL.Query().Get("error")
		if errParam != "" {
			fmt.Fprintf(w, "<html><body><h2>❌ Authorization denied: %s</h2><p>You can close this tab.</p></body></html>", errParam)
			errCh <- fmt.Errorf("user denied authorization: %s", errParam)
			return
		}
		fmt.Fprintf(w, "<html><body><h2>✅ Authorized!</h2><p>You can close this tab and return to the terminal.</p></body></html>")
		codeCh <- code
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	fmt.Print("  Waiting for Spotify authorization")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var code string
	select {
	case code = <-codeCh:
		fmt.Println(" ✅")
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return fmt.Errorf("timed out waiting for Spotify authorization")
	}

	_ = srv.Shutdown(context.Background())

	// Exchange code for tokens
	fmt.Print("  Exchanging code for tokens... ")
	accessToken, refreshToken, expiresIn, err := exchangeSpotifyCode(
		cfg.SpotifyClientID, cfg.SpotifyClientSecret, code, spotifyRedirectURI,
	)
	if err != nil {
		fmt.Println("❌")
		return err
	}
	fmt.Println("✅")

	cfg.SpotifyAccessToken = accessToken
	cfg.SpotifyRefreshToken = refreshToken
	cfg.SpotifyTokenExpiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
	return nil
}

func prompt(label string) string {
	fmt.Print(label)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func openBrowser(u string) {
	_ = exec.Command("open", u).Start() // macOS
}
