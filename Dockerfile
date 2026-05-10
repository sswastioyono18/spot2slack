# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod .
# Download deps (none currently, but keeps layer cache if you add any later)
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /spotslack .

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
# ca-certificates needed for HTTPS calls to Spotify/Slack APIs
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /spotslack /usr/local/bin/spotslack

# Config dir (mounted from host)
VOLUME ["/config"]

# Port for Spotify OAuth callback (only needed during setup)
EXPOSE 8888

ENTRYPOINT ["spotslack"]
CMD []
