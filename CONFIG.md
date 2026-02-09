# Configuration Guide

The broadcast relay is configured using environment variables. Below are all available configuration options:

## Configuration

### SEED_RELAYS
**Default:** `ws://localhost:10547`

Comma-separated list of seed relay URLs. These relays will be used for initial discovery and periodic refresh.

The default uses the nak debug relay running on localhost. For production, you should set this to real Nostr relays.

Example:
```bash
export SEED_RELAYS="wss://relay.damus.io,wss://relay.nostr.band,wss://nos.lol,wss://relay.snort.social"
```

### MANDATORY_RELAYS
**Default:** none

Comma-separated list of mandatory relay URLs. These relays will ALWAYS receive broadcasts, regardless of their score or ranking.

This is useful for ensuring events reach specific relays, such as:
- Your own personal relay
- Backup relays you control
- Critical infrastructure relays

Mandatory relays are added in addition to the top N relays selected by the scoring algorithm.

Example:
```bash
export MANDATORY_RELAYS="wss://my-relay.com,wss://backup-relay.com"
```

## Optional Configuration

### TOP_N_RELAYS
**Default:** `50`

Number of top relays to broadcast events to. Higher numbers provide more coverage but require more bandwidth and connections.

Example:
```bash
export TOP_N_RELAYS=100
```

### RELAY_PORT
**Default:** `3334`

Port for the relay server to listen on.

Example:
```bash
export RELAY_PORT=3334
```

### REFRESH_INTERVAL
**Default:** `24h`

How often to refresh the relay list from seed relays. Format is a duration string (e.g., "24h", "12h", "1h30m").

Example:
```bash
export REFRESH_INTERVAL=24h
```

### HEALTH_CHECK_INTERVAL
**Default:** `5m`

How often to perform health checks on relays. Format is a duration string (e.g., "5m", "10m", "1h").

Example:
```bash
export HEALTH_CHECK_INTERVAL=5m
```

### INITIAL_TIMEOUT
**Default:** `5s`

Timeout for initial relay testing during discovery. Format is a duration string (e.g., "5s", "10s").

Example:
```bash
export INITIAL_TIMEOUT=5s
```

### SUCCESS_RATE_DECAY
**Default:** `0.95`

Decay factor for exponential moving average of success rate. Range: 0.0 to 1.0. Higher values give more weight to historical data.

Example:
```bash
export SUCCESS_RATE_DECAY=0.95
```

## Quick Start

1. (Optional) Set your seed relays. The default is `ws://localhost:10547` (nak debug relay):
```bash
export SEED_RELAYS="wss://relay.damus.io,wss://relay.nostr.band,wss://nos.lol"
```

2. (Optional) Configure other settings:
```bash
export TOP_N_RELAYS=50
export RELAY_PORT=3334
```

3. Run the relay:
```bash
./broadcast-relay
```

## Endpoints

- **WebSocket:** `ws://localhost:3334/` - Main relay endpoint
- **Stats:** `http://localhost:3334/stats` - JSON endpoint showing current relay statistics

### VERBOSE
**Default:** none

Enable verbose logging. This environment variable has the same effect as the `--verbose` command line option. Command line options override environment variables.

Examples:
- `VERBOSE="all"` or `VERBOSE="true"` - Enable all debug logging
- `VERBOSE="discovery,health"` - Enable debug logging for specific modules
- `VERBOSE="broadcaster.addEventToCache"` - Enable debug logging for specific methods

Example:
```bash
export VERBOSE="discovery,health"
```

## Example Stats Response

```json
{
  "total_relays": 150,
  "active_relays": 50,
  "top_relays": [
    {
      "url": "wss://relay.damus.io",
      "success_rate": 0.9850,
      "avg_response_ms": 120,
      "total_attempts": 1000
    }
  ]
}
```

