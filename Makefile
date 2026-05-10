IMAGE   := spotslack
COMPOSE := docker compose
CONFIG  := $(HOME)/.config/spotslack

.PHONY: build setup run stop restart logs status clear shell help

## Build the Docker image
build:
	$(COMPOSE) build

## First-time OAuth setup (opens browser, catches Spotify callback on :8888)
setup: build
	$(COMPOSE) run --rm --profile setup setup

## Start the daemon in the background
run: build
	$(COMPOSE) --profile daemon up -d daemon
	@echo "Daemon running. Logs: make logs"

## Stop the daemon
stop:
	$(COMPOSE) --profile daemon down

## Restart the daemon
restart: stop run

## Tail daemon logs
logs:
	docker logs -f spotslack

## Show current Spotify + Slack status
status:
	$(COMPOSE) run --rm --profile status status

## Clear Slack status immediately
clear:
	$(COMPOSE) run --rm --profile clear clear

## Drop into a shell inside the container (debug)
shell:
	docker run --rm -it \
		-v $(CONFIG):/config \
		-e SPOTSLACK_CONFIG=/config/config.json \
		$(IMAGE) sh

## Print this help
help:
	@grep -E '^## ' Makefile | sed 's/## //'
