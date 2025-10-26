#!/bin/bash

# Example script to run the broadcast relay
# Copy this file and modify the settings to match your needs

# Required: Set your seed relays
export SEED_RELAYS="wss://relay.damus.io,wss://relay.nostr.band,wss://nos.lol,wss://relay.snort.social,wss://relay.primal.net"

# Optional: Configure other settings
export TOP_N_RELAYS=50
export RELAY_PORT=3334
export REFRESH_INTERVAL=24h
export HEALTH_CHECK_INTERVAL=5m
export INITIAL_TIMEOUT=5s
export SUCCESS_RATE_DECAY=0.95
export MANDATORY_RELAYS="wss://relay.damus.io,wss://relay.nostr.band,wss://nos.lol,wss://relay.snort.social,wss://relay.primal.net"

# Optional: Enable verbose logging via environment variable
# You can also use --verbose command line option to override this
export VERBOSE="discovery,health"

# Build and run
echo "Building broadcast-relay..."
go build -o broadcast-relay

echo "Starting broadcast relay..."
echo "WebSocket endpoint: ws://localhost:$RELAY_PORT"
echo "Stats endpoint: http://localhost:$RELAY_PORT/stats"
echo ""
echo "Note: VERBOSE environment variable is set to '$VERBOSE'"
echo "You can override it with: ./broadcast-relay --verbose=all"
echo ""
./broadcast-relay

