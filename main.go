package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const pollInterval = 15 * time.Second

func main() {
	// Support subcommands: setup, run (default), status, clear
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "setup":
			if err := runSetup(); err != nil {
				log.Fatalf("Setup failed: %v", err)
			}
			return
		case "status":
			printStatus()
			return
		case "clear":
			cfg, err := loadConfig()
			if err != nil {
				log.Fatalf("Config error: %v", err)
			}
			if err := clearSlackStatus(cfg.SlackToken); err != nil {
				log.Fatalf("Failed to clear status: %v", err)
			}
			fmt.Println("Slack status cleared.")
			return
		case "help", "--help", "-h":
			printHelp()
			return
		}
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Config not found. Run: spotslack setup\n")
		os.Exit(1)
	}

	log.Printf("🎵 spotslack starting (poll every %s)", pollInterval)
	log.Printf("   Slack workspace: %s", cfg.SlackTeam)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		log.Println("Shutting down, clearing Slack status...")
		_ = clearSlackStatus(cfg.SlackToken)
		cancel()
	}()

	runDaemon(ctx, cfg)
}

func runDaemon(ctx context.Context, cfg *Config) {
	var lastStatusText string  // track what we set, so we don't clear manual statuses
	var weSetStatus bool

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Run immediately on start
	weSetStatus, lastStatusText = tick(cfg, weSetStatus, lastStatusText)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			weSetStatus, lastStatusText = tick(cfg, weSetStatus, lastStatusText)
		}
	}
}

func tick(cfg *Config, weSetStatus bool, lastStatusText string) (bool, string) {
	// Refresh Spotify token if needed
	if err := cfg.RefreshSpotifyTokenIfNeeded(); err != nil {
		log.Printf("⚠️  Spotify token refresh failed: %v", err)
		return weSetStatus, lastStatusText
	}

	track, playing, err := getCurrentTrack(cfg.SpotifyAccessToken)
	if err != nil {
		log.Printf("⚠️  Spotify API error: %v", err)
		return weSetStatus, lastStatusText
	}

	if playing {
		statusText := fmt.Sprintf("%s — %s", track.Name, track.Artist)
		if statusText == lastStatusText {
			return weSetStatus, lastStatusText // no change
		}

		emoji := cfg.nextEmoji()
		if err := setSlackStatus(cfg.SlackToken, emoji, statusText); err != nil {
			log.Printf("⚠️  Slack update failed: %v", err)
			return weSetStatus, lastStatusText
		}
		log.Printf("✅ %s %s", emoji, statusText)
		return true, statusText
	}

	// Not playing — clear only if we set it
	if weSetStatus {
		// Check current Slack status first; user may have changed it manually
		current, err := getSlackStatus(cfg.SlackToken)
		if err == nil && current == lastStatusText {
			if err := clearSlackStatus(cfg.SlackToken); err != nil {
				log.Printf("⚠️  Failed to clear Slack status: %v", err)
			} else {
				log.Println("⏹️  Playback stopped — Slack status cleared")
			}
		}
		return false, ""
	}

	return false, lastStatusText
}

func printStatus() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("Not configured. Run: spotslack setup")
		return
	}
	if err := cfg.RefreshSpotifyTokenIfNeeded(); err != nil {
		fmt.Printf("Spotify token error: %v\n", err)
		return
	}
	track, playing, err := getCurrentTrack(cfg.SpotifyAccessToken)
	if err != nil {
		fmt.Printf("Spotify API error: %v\n", err)
		return
	}
	if playing {
		fmt.Printf("🎵 Now playing: %s — %s\n", track.Name, track.Artist)
		if track.Album != "" {
			fmt.Printf("   Album: %s\n", track.Album)
		}
	} else {
		fmt.Println("⏹️  Nothing playing")
	}

	slackStatus, err := getSlackStatus(cfg.SlackToken)
	if err != nil {
		fmt.Printf("Slack error: %v\n", err)
		return
	}
	if slackStatus != "" {
		fmt.Printf("💬 Slack status: %s\n", slackStatus)
	} else {
		fmt.Println("💬 Slack status: (empty)")
	}
}

func printHelp() {
	fmt.Println(`spotslack — sync Spotify → Slack status

USAGE:
  spotslack           Run the daemon (polls every 15s)
  spotslack setup     Interactive OAuth setup wizard
  spotslack status    Show current Spotify track and Slack status
  spotslack clear     Clear your Slack status immediately

SETUP:
  1. Create a Spotify app at https://developer.spotify.com/dashboard
     Set redirect URI: http://127.0.0.1:8888/callback
  2. Create a Slack app at https://api.slack.com/apps
     Add OAuth scope: users.profile:write, users.profile:read
  3. Run: spotslack setup

CONFIG FILE: ~/.config/spotslack/config.json

AUTOSTART (launchd):
  cp com.spotslack.plist ~/Library/LaunchAgents/
  launchctl load ~/Library/LaunchAgents/com.spotslack.plist`)
}

// Config serialization helpers used by setup.go
func saveConfigToFile(cfg *Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
