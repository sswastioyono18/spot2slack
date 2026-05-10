# spotslack

Syncs your Spotify "now playing" to Slack status. Runs entirely locally in Docker — no external server, no cloud dependency.

```
🎧 Bohemian Rhapsody — Queen
```

## Prerequisites

- Docker + Docker Compose
- A Spotify account (free or Premium)
- A Slack workspace where you can install apps

## Quick start

```bash
cp .env.example .env   # adjust TZ if needed
make setup             # one-time OAuth wizard
make run               # start daemon in background
make logs              # tail live output
```

## Setup

### 1. Spotify app
1. https://developer.spotify.com/dashboard → Create app
2. Add redirect URI: `http://localhost:8888/callback`
3. Copy Client ID and Client Secret

### 2. Slack app
1. https://api.slack.com/apps → Create New App → From scratch
2. OAuth & Permissions → User Token Scopes, add:
   - `users.profile:write`
   - `users.profile:read`
   - `team:read`
3. Install to Workspace → copy the User OAuth Token (xoxp-…)

### 3. Wizard
```bash
make setup
```
Opens browser for Spotify auth (callback on localhost:8888), prompts for Slack token, saves to `~/.config/spotslack/config.json`.

## Commands

| Command         | What it does                              |
|----------------|-------------------------------------------|
| `make build`   | Build Docker image                        |
| `make setup`   | One-time OAuth setup                      |
| `make run`     | Start daemon in background                |
| `make stop`    | Stop daemon                               |
| `make restart` | Restart daemon                            |
| `make logs`    | Tail live logs                            |
| `make status`  | Show current track + Slack status         |
| `make clear`   | Immediately clear Slack status            |
| `make shell`   | Drop into container shell (debug)         |

## Config: `~/.config/spotslack/config.json`

```json
{
  "spotify_client_id": "...",
  "spotify_client_secret": "...",
  "spotify_refresh_token": "...",
  "slack_token": "xoxp-...",
  "slack_team": "<Some Team>",
  "emojis": ["🎧", "🎵", "🎶"]
}
```

## Behavior

- Polls Spotify every 15s
- Sets status: `{emoji} Track — Artist`
- Clears status on stop — only if spotslack set it (manual statuses are safe)
- Spotify token auto-refreshed, written back to config
- Container restarts automatically (`restart: unless-stopped`)
