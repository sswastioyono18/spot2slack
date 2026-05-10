package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type slackProfileSetRequest struct {
	Profile slackProfile `json:"profile"`
}

type slackProfile struct {
	StatusText       string `json:"status_text"`
	StatusEmoji      string `json:"status_emoji"`
	StatusExpiration int    `json:"status_expiration"` // 0 = no expiry
}

// setSlackStatus updates the authenticated user's Slack status.
func setSlackStatus(token, emoji, text string) error {
	payload := slackProfileSetRequest{
		Profile: slackProfile{
			StatusText:       text,
			StatusEmoji:      emojiToColonForm(emoji),
			StatusExpiration: 0,
		},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "https://slack.com/api/users.profile.set", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("slack request: %w", err)
	}
	defer resp.Body.Close()

	return checkSlackResponse(resp.Body)
}

// clearSlackStatus removes the user's Slack status.
func clearSlackStatus(token string) error {
	return setSlackStatus(token, "", "")
}

// getSlackStatus returns the current status text of the authenticated user.
func getSlackStatus(token string) (string, error) {
	req, _ := http.NewRequest("GET", "https://slack.com/api/users.profile.get", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("slack request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK      bool   `json:"ok"`
		Error   string `json:"error"`
		Profile struct {
			StatusText string `json:"status_text"`
		} `json:"profile"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode slack response: %w", err)
	}
	if !result.OK {
		return "", fmt.Errorf("slack error: %s", result.Error)
	}
	return result.Profile.StatusText, nil
}

// getSlackTeamName returns the workspace name for display purposes.
func getSlackTeamName(token string) (string, error) {
	req, _ := http.NewRequest("GET", "https://slack.com/api/team.info", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		OK   bool   `json:"ok"`
		Team struct {
			Name string `json:"name"`
		} `json:"team"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if !result.OK {
		return "", fmt.Errorf("slack error: %s", result.Error)
	}
	return result.Team.Name, nil
}

func checkSlackResponse(body io.Reader) error {
	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return fmt.Errorf("decode slack response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("slack API error: %s", result.Error)
	}
	return nil
}

// emojiToColonForm converts a Unicode emoji or :name: string to Slack's :name: format.
// For Unicode emoji we pass it directly; Slack accepts both formats.
func emojiToColonForm(emoji string) string {
	if emoji == "" {
		return ""
	}
	// Already in :name: form
	if len(emoji) > 2 && emoji[0] == ':' && emoji[len(emoji)-1] == ':' {
		return emoji
	}
	// Unicode emoji — Slack accepts these directly in status_emoji field
	return emoji
}
