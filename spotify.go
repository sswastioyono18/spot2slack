package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Track struct {
	Name   string
	Artist string
	Album  string
	ID     string
}

type spotifyCurrentlyPlayingResponse struct {
	IsPlaying bool `json:"is_playing"`
	Item      struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Album struct {
			Name string `json:"name"`
		} `json:"album"`
		Artists []struct {
			Name string `json:"name"`
		} `json:"artists"`
	} `json:"item"`
	CurrentlyPlayingType string `json:"currently_playing_type"` // "track", "episode"
}

// getCurrentTrack fetches the currently playing Spotify track.
// Returns (track, isPlaying, error). isPlaying=false when nothing is playing.
func getCurrentTrack(accessToken string) (*Track, bool, error) {
	req, _ := http.NewRequest("GET", "https://api.spotify.com/v1/me/player/currently-playing", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("spotify request: %w", err)
	}
	defer resp.Body.Close()

	// 204 = nothing playing
	if resp.StatusCode == http.StatusNoContent {
		return nil, false, nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, false, fmt.Errorf("spotify unauthorized (token expired?)")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, false, fmt.Errorf("spotify API %d: %s", resp.StatusCode, string(body))
	}

	var result spotifyCurrentlyPlayingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, false, fmt.Errorf("decode spotify response: %w", err)
	}

	if !result.IsPlaying {
		return nil, false, nil
	}

	// Build artist string (comma-join for multi-artist tracks)
	artists := make([]string, len(result.Item.Artists))
	for i, a := range result.Item.Artists {
		artists[i] = a.Name
	}
	artistStr := strings.Join(artists, ", ")

	track := &Track{
		ID:     result.Item.ID,
		Name:   result.Item.Name,
		Artist: artistStr,
		Album:  result.Item.Album.Name,
	}
	return track, true, nil
}

// refreshSpotifyToken uses the refresh token to get a new access token.
// Returns (accessToken, expiresInSeconds, error).
func refreshSpotifyToken(clientID, clientSecret, refreshToken string) (string, int, error) {
	creds := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	req, _ := http.NewRequest("POST", "https://accounts.spotify.com/api/token",
		strings.NewReader(form.Encode()))
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("token refresh request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", 0, fmt.Errorf("decode token response: %w", err)
	}
	if result.Error != "" {
		return "", 0, fmt.Errorf("spotify token error: %s", result.Error)
	}
	return result.AccessToken, result.ExpiresIn, nil
}

// exchangeSpotifyCode exchanges an auth code for tokens (used during setup).
func exchangeSpotifyCode(clientID, clientSecret, code, redirectURI string) (accessToken, refreshToken string, expiresIn int, err error) {
	creds := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	req, _ := http.NewRequest("POST", "https://accounts.spotify.com/api/token",
		strings.NewReader(form.Encode()))
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", 0, fmt.Errorf("spotify exchange: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", 0, fmt.Errorf("decode exchange: %w", err)
	}
	if result.Error != "" {
		return "", "", 0, fmt.Errorf("spotify error: %s — %s", result.Error, result.ErrorDesc)
	}
	return result.AccessToken, result.RefreshToken, result.ExpiresIn, nil
}
