# Broadcast Relay

A high-performance Nostr relay that receives events and broadcasts them to the top N fastest and most reliable relays in the network. Built with [khatru](https://github.com/fiatjaf/khatru).

## Features

- **Zero Storage**: Pure relay that doesn't store events, only broadcasts them
- **Intelligent Relay Selection**: Automatically discovers and ranks relays based on:
  - Response time
  - Success rate (with exponential decay)
  - Composite scoring algorithm
- **Automatic Discovery**: Finds new relays from:
  - Seed relays
  - Kind 3 events (contact lists)
  - Kind 10002 events (relay list metadata)
  - Relay hints in event tags
- **Health Monitoring**: 
  - Initial timeout-based testing for new relays
  - Continuous success rate tracking with exponential decay
  - Automatic demotion of unreliable relays
- **Concurrent Broadcasting**: Events are broadcast to top N relays simultaneously
- **Periodic Refresh**: Relay list refreshed every 24 hours (configurable)
- **Statistics Endpoint**: Real-time stats about relay performance

## Architecture

```
┌─────────────────┐
│  Nostr Client   │
│  (publishes)    │
└────────┬────────┘
         │
         v
┌─────────────────────────────────────┐
│       Broadcast Relay (This)        │
│  ┌──────────┐    ┌───────────────┐  │
│  │  Relay   │───▶│  Discovery    │  │
│  │  Server  │    │  (Extract     │  │
│  │(Khatru)  │    │   relays)     │  │
│  └────┬─────┘    └───────────────┘  │
│       │                              │
│       v                              │
│  ┌──────────────────┐                │
│  │   Broadcaster    │                │
│  └────┬─────────────┘                │
│       │                              │
│  ┌────v─────────────┐                │
│  │  Relay Manager   │                │
│  │  (Ranking &      │                │
│  │   Selection)     │                │
│  └──────────────────┘                │
└─────────────────────────────────────┘
         │
         v
┌─────────────────────────┐
│  Top N Relays           │
│  (Concurrent Publish)   │
│  - wss://relay1.com     │
│  - wss://relay2.com     │
│  - ...                  │
└─────────────────────────┘
```

## Installation

### Prerequisites

- Go 1.21 or higher

### Build from Source

```bash
git clone https://github.com/girino/broadcast-relay.git
cd broadcast-relay
go build -o broadcast-relay
```

## Configuration

Configure the relay using environment variables. See [CONFIG.md](CONFIG.md) for detailed configuration options.

### Required Configuration

```bash
export SEED_RELAYS="wss://relay.damus.io,wss://relay.nostr.band,wss://nos.lol"
```

### Optional Configuration

```bash
export TOP_N_RELAYS=50              # Number of top relays to broadcast to (default: 50)
export RELAY_PORT=3334              # Port to listen on (default: 3334)
export REFRESH_INTERVAL=24h         # Relay list refresh interval (default: 24h)
export HEALTH_CHECK_INTERVAL=5m     # Health check interval (default: 5m)
export INITIAL_TIMEOUT=5s           # Initial relay test timeout (default: 5s)
export SUCCESS_RATE_DECAY=0.95      # Exponential decay factor (default: 0.95)
```

## Usage

### Starting the Relay

```bash
export SEED_RELAYS="wss://relay.damus.io,wss://relay.nostr.band,wss://nos.lol"
./broadcast-relay
```

### Connecting Clients

Connect your Nostr client to:
```
ws://localhost:3334
```

### Viewing Statistics

View real-time relay statistics:
```bash
curl http://localhost:3334/stats
```

Example response:
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

## How It Works

### 1. Initialization

1. Loads configuration from environment variables
2. Initializes relay manager with ranking system
3. Discovers relays from seed relays
4. Tests all discovered relays with timeout-based checks
5. Selects top N relays based on initial performance
6. Switches to exponential decay mode for ongoing tracking

### 2. Event Processing

When an event is received:

1. **Extract Relay URLs**: Parse the event for:
   - Relay hints in tags (e.g., `["e", "<id>", "<relay-url>"]`)
   - Kind 3 events (contact lists with relay info)
   - Kind 10002 events (relay list metadata)

2. **Add New Relays**: Newly discovered relays are:
   - Added to the relay pool
   - Tested asynchronously
   - Ranked based on performance

3. **Broadcast**: Event is broadcast concurrently to top N relays

4. **Update Rankings**: Results from each broadcast update relay rankings:
   - Success rate updated with exponential decay
   - Average response time updated with moving average
   - Composite score recalculated

### 3. Relay Ranking Algorithm

Relays are ranked using a composite score:

```
score = (successRate × 100) - (avgResponseTime.seconds × 10)
```

Where:
- `successRate`: Exponentially decayed success rate (0.0 to 1.0)
- `avgResponseTime`: Exponential moving average of response times

The exponential decay formula:
```
newSuccessRate = oldSuccessRate × decay + currentResult × (1 - decay)
```

This ensures:
- Recent performance has more weight
- Historical performance still matters
- Fast, reliable relays rank highest

### 4. Periodic Refresh

Every 24 hours (configurable):
1. Re-discover relays from seed relays
2. Test new relays
3. Update rankings
4. Re-select top N relays

## Project Structure

```
broadcast-relay/
├── main.go                 # Entry point and lifecycle management
├── config/
│   └── config.go          # Environment variable configuration
├── manager/
│   └── manager.go         # Relay ranking, selection, and health tracking
├── health/
│   └── health.go          # Health checking and testing
├── discovery/
│   └── discovery.go       # Relay discovery from seeds and events
├── broadcaster/
│   └── broadcaster.go     # Event broadcasting to top N relays
├── relay/
│   └── relay.go           # Khatru relay server setup
├── CONFIG.md              # Detailed configuration documentation
└── README.md              # This file
```

## Dependencies

- [khatru](https://github.com/fiatjaf/khatru) - Relay framework
- [go-nostr](https://github.com/nbd-wtf/go-nostr) - Nostr protocol implementation

## Performance Considerations

- **Concurrent Broadcasting**: Each event is broadcast to N relays in parallel
- **In-Memory Only**: No disk I/O for event storage
- **Efficient Ranking**: O(n log n) sorting for relay selection
- **Batch Testing**: Initial relay tests run concurrently with semaphore limiting

## Use Cases

1. **Maximum Reach**: Broadcast important events to as many relays as possible
2. **Reliability**: Automatically avoid unreliable relays
3. **Performance**: Always use the fastest relays
4. **Discovery**: Help clients discover new, high-quality relays

## Limitations

- No event storage (by design)
- No REQ query support (returns empty results)
- No event deletion (nothing to delete)
- Requires at least one working seed relay to start

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License

## Author

Created by Girino Vey

## Acknowledgments

- [fiatjaf](https://github.com/fiatjaf) for khatru
- The Nostr community for the protocol and ecosystem

