package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	// Spotify OAuth
	SpotifyClientID     string    `json:"spotify_client_id"`
	SpotifyClientSecret string    `json:"spotify_client_secret"`
	SpotifyRefreshToken string    `json:"spotify_refresh_token"`
	SpotifyAccessToken  string    `json:"spotify_access_token"`
	SpotifyTokenExpiry  time.Time `json:"spotify_token_expiry"`

	// Slack
	SlackToken string `json:"slack_token"`
	SlackTeam  string `json:"slack_team,omitempty"`

	// Preferences
	Emojis       []string `json:"emojis"`
	emojiIndex   int
}

func configPath() string {
	if p := os.Getenv("SPOTSLACK_CONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "spotslack", "config.json")
}

func loadConfig() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if len(cfg.Emojis) == 0 {
		cfg.Emojis = []string{"🎧", "🎵", "🎶"}
	}
	if cfg.SlackToken == "" {
		return nil, fmt.Errorf("slack_token not set in config")
	}
	if cfg.SpotifyClientID == "" || cfg.SpotifyClientSecret == "" {
		return nil, fmt.Errorf("spotify credentials not set in config")
	}
	return &cfg, nil
}

func (c *Config) save() error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (c *Config) nextEmoji() string {
	emoji := c.Emojis[c.emojiIndex%len(c.Emojis)]
	c.emojiIndex++
	return emoji
}

// RefreshSpotifyTokenIfNeeded gets a new access token when within 60s of expiry.
func (c *Config) RefreshSpotifyTokenIfNeeded() error {
	if time.Now().Before(c.SpotifyTokenExpiry.Add(-60 * time.Second)) {
		return nil // still valid
	}
	accessToken, expiresIn, err := refreshSpotifyToken(c.SpotifyClientID, c.SpotifyClientSecret, c.SpotifyRefreshToken)
	if err != nil {
		return err
	}
	c.SpotifyAccessToken = accessToken
	c.SpotifyTokenExpiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
	return c.save()
}
