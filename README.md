# Broadcast Relay

<div align="center">

**A high-performance Nostr relay that broadcasts your events to the best relays in the network**

[![License: GAL](https://img.shields.io/badge/license-GAL-purple)](https://license.girino.org)
[![Go Version](https://img.shields.io/badge/go-1.23%2B-blue)](https://go.dev/)
[![Docker](https://img.shields.io/badge/docker-ready-blue)](https://www.docker.com/)

[Features](#features) • [Quick Start](#quick-start) • [Documentation](#documentation) • [Contributing](#contributing)

</div>

---

## What is Broadcast Relay?

Broadcast Relay is a **zero-storage Nostr relay** that accepts your events and intelligently broadcasts them to multiple high-quality relays. Instead of storing events, it focuses on maximizing reach and reliability by:

- Discovering and testing relays automatically
- Ranking relays by performance and reliability
- Broadcasting to the top-N relays simultaneously
- Continuous health monitoring and re-ranking

## Features

### Core Functionality
- 🚀 **Zero Storage** - Pure broadcast relay, no database required
- 🎯 **Smart Relay Selection** - Automatically ranks relays by speed and reliability
- 🔄 **Auto-Discovery** - Finds relays from seeds, contact lists, and relay metadata
- ⚡ **Worker Pool Architecture** - Concurrent event processing with configurable workers
- 📊 **Real-time Stats** - Live monitoring via HTTP endpoint
- 🔐 **Duplicate Prevention** - Event deduplication cache with TTL
- 💪 **Overflow Queue** - Hybrid channel + unbounded queue handles traffic spikes

### Advanced Features
- 🔍 **Granular Logging** - Module and method-level verbose control
- 🏥 **Health Monitoring** - Continuous relay health checks
- 📈 **Performance Metrics** - Queue stats, cache hits/misses, saturation tracking
- 🎨 **Beautiful UI** - Modern web interface with relay information
- 🧅 **Tor Support** - Docker setup includes hidden service
- 🔧 **Highly Configurable** - Environment variables for all settings

### NIP Support
- ✅ NIP-01: Basic protocol flow
- ✅ NIP-11: Relay information document

## Quick Start

### Using Docker (Recommended)

```bash
# Clone the repository
git clone nostr://npub18lav8fkgt8424rxamvk8qq4xuy9n8mltjtgztv2w44hc5tt9vets0hcfsz/relay.ngit.dev/broadcast-relay
cd broadcast-relay

# Or via HTTPS
git clone https://gitworkshop.dev/girino@girino.org/broadcast-relay
cd broadcast-relay

# Start the relay (includes Tor hidden service)
docker-compose -f docker-compose.prod.yml up -d

# Get your Tor .onion address (wait ~60s)
docker exec broadcast-relay-tor cat /var/lib/tor/hidden_service/relay/hostname

# View logs
docker-compose -f docker-compose.prod.yml logs -f

# Check stats
curl http://localhost:3334/stats
```

Your relay is now running! 🎉

### Without Docker

```bash
# Prerequisites: Go 1.23+
go version

# Clone and build
git clone https://gitworkshop.dev/girino@girino.org/broadcast-relay.git
cd nostr-brodcast-relay
go build -o broadcast-relay .

# Run with defaults
./broadcast-relay

# Run with verbose logging
./broadcast-relay --verbose all

# Run with custom config
RELAY_PORT=8080 TOP_N_RELAYS=100 ./broadcast-relay
```

## Documentation

### 📚 Comprehensive Guides

- **[CONFIG.md](CONFIG.md)** - Complete configuration reference
- **[DOCKER.md](DOCKER.md)** - Docker deployment with Tor
- **[VERBOSE_LOGGING.md](VERBOSE_LOGGING.md)** - Debugging and logging guide

### ⚙️ Configuration

The relay is configured entirely through environment variables. Copy `example.env` and customize:

```bash
cp example.env .env
nano .env
```

**Key Configuration Options:**

| Variable | Description | Default |
|----------|-------------|---------|
| `SEED_RELAYS` | Initial relays for discovery | `wss://relay.damus.io,...` |
| `TOP_N_RELAYS` | Number of relays to broadcast to | `50` |
| `WORKER_COUNT` | Broadcast workers (0=auto 2×CPU) | `0` |
| `CACHE_TTL` | Event deduplication cache duration | `5m` |
| `RELAY_URL` | Your public relay address | `ws://localhost:3334` |
| `RELAY_NAME` | Relay name | `Broadcast Relay` |
| `CONTACT_PUBKEY` | Contact npub/hex | Auto-generated |
| `RELAY_PRIVKEY` | Relay nsec (keep secret!) | Auto-generated |

**See [example.env](example.env) for all 20+ configuration options.**

### 🐳 Docker Deployment

Full production setup with Tor hidden service:

```bash
# Quick start
docker-compose -f docker-compose.prod.yml up -d

# With custom config
cp example.env .env
# Edit .env with your settings
docker-compose -f docker-compose.prod.yml up -d

# Get your .onion address
docker exec broadcast-relay-tor cat /var/lib/tor/hidden_service/relay/hostname
```

**Included Services:**
- Broadcast Relay (main service)
- Tor Hidden Service (automatic .onion)
- Autoheal (automatic container recovery)

**See [DOCKER.md](DOCKER.md) for complete Docker guide.**

### 🌐 Nginx Setup (Optional)

For clearnet access with SSL:

```bash
# Copy nginx config
sudo cp nginx.conf.example /etc/nginx/sites-available/broadcast-relay
sudo ln -s /etc/nginx/sites-available/broadcast-relay /etc/nginx/sites-enabled/

# Get SSL certificate (automatic HTTPS setup)
sudo certbot --nginx -d relay.example.com

# Nginx proxies to localhost:3334 (from Docker)
```

### 📊 Monitoring

**Stats Endpoint:** `http://localhost:3334/stats`

Returns JSON with:
- Queue metrics (size, capacity, saturation)
- Cache statistics (hits, misses, hit rate)
- Worker count and status
- Top relay list with scores
- Mandatory relay performance

**Main Page:** `http://localhost:3334/`

Beautiful web interface showing:
- Relay information and branding
- Connection details
- How the relay works
- Links to stats and documentation

## Building from Source

### Prerequisites

- **Go 1.23+** (for go.mod compatibility)
- Git

### Build Steps

```bash
# Clone repository
git clone https://gitworkshop.dev/girino@girino.org/broadcast-relay.git
cd nostr-brodcast-relay

# Download dependencies
go mod download

# Build
go build -o broadcast-relay .

# Run
./broadcast-relay

# Build for different platforms
GOOS=linux GOARCH=amd64 go build -o broadcast-relay-linux-amd64 .
GOOS=darwin GOARCH=arm64 go build -o broadcast-relay-darwin-arm64 .
GOOS=windows GOARCH=amd64 go build -o broadcast-relay-windows-amd64.exe .
```

### Development Build

```bash
# Build with race detector
go build -race -o broadcast-relay .

# Run with verbose logging for development
./broadcast-relay --verbose all

# Run with specific module debugging
./broadcast-relay --verbose "broadcaster,health"
```

## Usage Examples

### Basic Usage

```bash
# Default configuration
./broadcast-relay

# Custom port
RELAY_PORT=8080 ./broadcast-relay

# Specific seed relays
SEED_RELAYS="wss://relay.damus.io,wss://nos.lol" ./broadcast-relay

# More workers for high traffic
WORKER_COUNT=64 ./broadcast-relay
```

### Advanced Usage

```bash
# Mandatory relays (always broadcast here)
MANDATORY_RELAYS="wss://my-relay.com" ./broadcast-relay

# Longer cache TTL (prevent duplicates for 1 hour)
CACHE_TTL=1h ./broadcast-relay

# Broadcast to more relays
TOP_N_RELAYS=100 ./broadcast-relay

# Verbose logging for specific components
./broadcast-relay --verbose "broadcaster.addEventToCache,health.CheckInitial"
```

### Production Example

```bash
# Full production configuration
RELAY_NAME="My Broadcast Relay" \
RELAY_URL="wss://relay.example.com" \
RELAY_DESCRIPTION="High-performance broadcast relay for Nostr" \
CONTACT_PUBKEY="npub1abc..." \
RELAY_PRIVKEY="nsec1xyz..." \
TOP_N_RELAYS=100 \
WORKER_COUNT=0 \
CACHE_TTL=5m \
./broadcast-relay
```

## Architecture

### System Overview

```
┌─────────────┐
│Nostr Client │
│ (publishes) │
└──────┬──────┘
       │
       ▼
┌──────────────────────────────┐
│    Broadcast Relay           │
│                              │
│  ┌─────────────────────┐    │
│  │  HTTP/WebSocket     │    │
│  │  Router             │    │
│  └──────┬──────────────┘    │
│         │                   │
│  ┌──────▼──────────────┐    │
│  │  Event Queue        │    │
│  │  (Channel+Overflow) │    │
│  └──────┬──────────────┘    │
│         │                   │
│  ┌──────▼──────────────┐    │
│  │  Worker Pool        │    │
│  │  (Configurable)     │    │
│  └──────┬──────────────┘    │
│         │                   │
│  ┌──────▼──────────────┐    │
│  │  Relay Manager      │    │
│  │  (Scoring & Ranks)  │    │
│  └──────┬──────────────┘    │
└─────────┼───────────────────┘
          │
          ▼
   ┌─────────────┐
   │Top N Relays │
   │ + Mandatory │
   └─────────────┘
```

### Components

- **HTTP/WebSocket Router** - Handles incoming connections, serves main page and stats
- **Event Queue** - Hybrid channel (10×workers) + overflow list for burst traffic
- **Worker Pool** - Configurable workers process events concurrently
- **Deduplication Cache** - Prevents duplicate broadcasts (time and size limited)
- **Relay Manager** - Discovers, scores, and ranks relays
- **Health Checker** - Tests and monitors relay performance
- **Discovery** - Extracts relay URLs from events and seeds

## Contributing

We welcome contributions! Here's how to get involved:

### Getting Started

1. **Fork the repository**
2. **Clone your fork:**
   ```bash
   git clone https://github.com/YOUR_USERNAME/nostr-brodcast-relay.git
   cd nostr-brodcast-relay
   ```
3. **Create a feature branch:**
   ```bash
   git checkout -b feature/amazing-feature
   ```

### Development Workflow

1. **Make your changes**
   - Follow existing code style
   - Use `DebugMethod("module", "method", "message")` for logging
   - Add comments for complex logic

2. **Test your changes**
   ```bash
   # Build
   go build -o broadcast-relay .
   
   # Test with verbose logging
   ./broadcast-relay --verbose "yourmodule"
   
   # Check for issues
   go vet ./...
   ```

3. **Commit your changes**
   ```bash
   git add .
   git commit -m "feat: add amazing feature"
   ```
   
   **Commit Message Format:**
   - `feat: new feature`
   - `fix: bug fix`
   - `docs: documentation`
   - `refactor: code refactoring`
   - `perf: performance improvement`
   - `test: testing`

4. **Push and create PR**
   ```bash
   git push origin feature/amazing-feature
   ```
   Then create a Pull Request on GitHub

### Code Guidelines

- **Modules:** Each major component has its own package
- **Logging:** Use `logging.DebugMethod(module, method, message)` for debug logs
- **Configuration:** All config via environment variables in `config/config.go`
- **Error Handling:** Always check errors, log appropriately
- **Thread Safety:** Use mutexes or atomic operations for shared state
- **Comments:** Document public functions and complex logic

### Project Structure

```
broadcast-relay/
├── main.go              # Application entry point
├── config/              # Configuration management
├── relay/               # HTTP/WebSocket server
├── broadcaster/         # Event broadcasting logic
├── manager/             # Relay ranking and selection
├── health/              # Health checking
├── discovery/           # Relay discovery
├── logging/             # Logging utilities
├── templates/           # HTML templates
├── static/              # Static assets (icons, banners)
└── notes/               # Internal docs (not in git)
```

### Adding New Features

1. **New Configuration:**
   - Add field to `config.Config` struct
   - Add to `Load()` function with `getEnv*()`
   - Document in `example.env`

2. **New Module:**
   - Create package folder
   - Use `logging.DebugMethod("modulename", "methodname", ...)`
   - Export only what's needed

3. **New Endpoint:**
   - Add handler in `relay/relay.go`
   - Update documentation

### Testing

```bash
# Build
go build -o broadcast-relay .

# Run with test configuration
SEED_RELAYS="ws://localhost:10547" \
RELAY_PORT=3334 \
./broadcast-relay --verbose all

# Test specific functionality
./broadcast-relay --verbose "broadcaster.addEventToCache"

# Monitor stats
curl http://localhost:3334/stats | jq
```

## Troubleshooting

### Common Issues

**Queue Saturation**
```bash
# Check queue stats
curl http://localhost:3334/stats | jq '.queue'

# Increase workers
WORKER_COUNT=64 ./broadcast-relay
```

**Relay Discovery Issues**
```bash
# Use verbose logging
./broadcast-relay --verbose "discovery,health"

# Check seed relays are accessible
./broadcast-relay --verbose "health.CheckInitial"
```

**Cache Issues**
```bash
# Monitor cache
curl http://localhost:3334/stats | jq '.cache'

# Adjust TTL
CACHE_TTL=10m ./broadcast-relay

# Debug cache operations
./broadcast-relay --verbose "broadcaster.addEventToCache"
```

## Performance Tuning

### Worker Pool

```bash
# Auto (2× CPU cores) - default
WORKER_COUNT=0

# Explicit worker count
WORKER_COUNT=32

# High traffic
WORKER_COUNT=128
```

### Relay Selection

```bash
# More coverage
TOP_N_RELAYS=100

# Faster responses (fewer relays)
TOP_N_RELAYS=20

# Add mandatory relays
MANDATORY_RELAYS="wss://my-relay.com,wss://backup.com"
```

### Cache Configuration

```bash
# Short-term dedup (5 minutes)
CACHE_TTL=5m

# Long-term dedup (1 hour)
CACHE_TTL=1h

# Very short (testing)
CACHE_TTL=30s
```

## Production Deployment

### Docker Deployment

```bash
# 1. Clone repository
git clone https://gitworkshop.dev/girino@girino.org/broadcast-relay.git
cd nostr-brodcast-relay

# 2. Configure environment
cp example.env .env
nano .env  # Edit your settings

# 3. Start services
docker-compose -f docker-compose.prod.yml up -d

# 4. Get Tor address
docker exec broadcast-relay-tor cat /var/lib/tor/hidden_service/relay/hostname

# 5. (Optional) Setup nginx on host
sudo cp nginx.conf.example /etc/nginx/sites-available/broadcast-relay
sudo ln -s /etc/nginx/sites-available/broadcast-relay /etc/nginx/sites-enabled/
sudo certbot --nginx -d relay.example.com
```

### Binary Deployment

```bash
# 1. Build for your platform
go build -o broadcast-relay .

# 2. Create systemd service
sudo nano /etc/systemd/system/broadcast-relay.service
```

**Example systemd service:**

```ini
[Unit]
Description=Nostr Broadcast Relay
After=network.target

[Service]
Type=simple
User=relay
WorkingDirectory=/opt/broadcast-relay
EnvironmentFile=/opt/broadcast-relay/.env
ExecStart=/opt/broadcast-relay/broadcast-relay
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

```bash
# 3. Enable and start
sudo systemctl daemon-reload
sudo systemctl enable broadcast-relay
sudo systemctl start broadcast-relay

# 4. Check status
sudo systemctl status broadcast-relay
journalctl -u broadcast-relay -f
```

## API Reference

### Stats Endpoint

**GET /stats**

Returns JSON with relay statistics:

```json
{
  "total_relays": 150,
  "active_relays": 50,
  "mandatory_relays": 2,
  "queue": {
    "worker_count": 32,
    "channel_size": 45,
    "channel_capacity": 320,
    "channel_utilization": 14.06,
    "overflow_size": 0,
    "total_queued": 45,
    "peak_size": 523,
    "saturation_count": 0,
    "is_saturated": false
  },
  "cache": {
    "size": 15234,
    "max_size": 100000,
    "utilization_pct": 15.23,
    "hits": 8523,
    "misses": 15234,
    "hit_rate_pct": 35.86
  },
  "mandatory_relay_list": [...],
  "top_relays": [...]
}
```

### NIP-11 Relay Information

**GET /** with `Accept: application/nostr+json`

Returns relay information document per NIP-11.

## Verbose Logging

Granular control over debug output:

```bash
# Enable all verbose logging
./broadcast-relay --verbose all

# Specific modules
./broadcast-relay --verbose "config,health,broadcaster"

# Specific methods
./broadcast-relay --verbose "broadcaster.addEventToCache"

# Mixed
./broadcast-relay --verbose "broadcaster.addEventToCache,health,main"
```

**Available modules:** `config`, `health`, `broadcaster`, `manager`, `discovery`, `relay`, `main`

**See [VERBOSE_LOGGING.md](VERBOSE_LOGGING.md) for complete guide.**

## Environment Variables Reference

### Relay Configuration
- `SEED_RELAYS` - Initial relays (comma-separated)
- `MANDATORY_RELAYS` - Always broadcast to these
- `TOP_N_RELAYS` - Number of top relays to use
- `RELAY_PORT` - WebSocket port

### Performance
- `WORKER_COUNT` - Broadcast workers (0=auto)
- `CACHE_TTL` - Event cache duration
- `REFRESH_INTERVAL` - Relay list refresh
- `SUCCESS_RATE_DECAY` - Scoring decay factor

### Relay Metadata
- `RELAY_NAME` - Your relay's name
- `RELAY_DESCRIPTION` - What it does
- `RELAY_URL` - Public WebSocket address
- `RELAY_PRIVKEY` - Relay private key (nsec)
- `CONTACT_PUBKEY` - Contact npub/hex
- `RELAY_ICON` - Icon URL
- `RELAY_BANNERS` - Comma-separated banner URLs

**Full reference:** [CONFIG.md](CONFIG.md)

## Project Status

### Current Version: 0.2.0-rc1

**Recent Features:**
- ✅ Worker pool architecture
- ✅ Event deduplication cache
- ✅ Time-based cache expiration
- ✅ Granular verbose logging
- ✅ Mandatory relay tracking
- ✅ Main page with relay info
- ✅ Docker production setup
- ✅ Tor hidden service support

### Roadmap

- [ ] Persistent relay statistics
- [ ] REST API for relay management
- [ ] Prometheus metrics export
- [ ] Rate limiting per client
- [ ] Event filtering rules
- [ ] Multi-relay connection pooling

## Contributing

### Ways to Contribute

1. **Code Contributions**
   - Bug fixes
   - New features
   - Performance improvements
   - Tests

2. **Documentation**
   - Improve guides
   - Add examples
   - Fix typos
   - Translate

3. **Testing & Feedback**
   - Report bugs
   - Suggest features
   - Share use cases
   - Performance testing

4. **Community**
   - Help others
   - Share knowledge
   - Write tutorials

### Contribution Process

1. Check [existing issues](https://gitworkshop.dev/girino@girino.org/broadcast-relay/issues)
2. Open an issue for discussion (for major changes)
3. Fork and create feature branch
4. Make changes with clear commits
5. Test thoroughly
6. Submit Pull Request
7. Respond to review feedback

### Code of Conduct

- Be respectful and constructive
- Welcome newcomers
- Focus on code quality
- Help others learn

## Security

### Reporting Security Issues

If you discover a security vulnerability:

1. **DO NOT** create a public issue
2. Email the maintainer (see contact info)
3. Provide details and reproduction steps
4. Allow time for fix before disclosure

### Security Best Practices

- **Keep `RELAY_PRIVKEY` secret** - Never commit to git
- **Use HTTPS/WSS** in production with proper TLS
- **Update regularly** - Pull latest security fixes
- **Monitor logs** - Watch for suspicious activity
- **Firewall** - Restrict access as needed

## FAQ

**Q: Does this relay store events?**  
A: No, it's a pure broadcast relay. Events are cached temporarily (default 5 minutes) only to prevent duplicates.

**Q: How many relays does it broadcast to?**  
A: Configurable via `TOP_N_RELAYS` (default 50) + any mandatory relays.

**Q: How are relays ranked?**  
A: By composite score of success rate and response time, with exponential decay.

**Q: Can I force broadcast to specific relays?**  
A: Yes, use `MANDATORY_RELAYS` for relays that always receive events.

**Q: What happens if queue fills up?**  
A: Overflow queue automatically grows (unbounded list). Check `/stats` for saturation.

**Q: How do I get an .onion address?**  
A: Use Docker setup - Tor hidden service is included and auto-configured.

**Q: Can I use my own icons/banners?**  
A: Yes, set `RELAY_ICON` and `RELAY_BANNERS` env vars with URLs or local paths.

## License

This project is licensed under the **Girino's Anarchist License (GAL)**.

Full license: https://license.girino.org

In brief: Do whatever you want with this code. No restrictions, no warranty.

## Links

- **Repository (Nostr):** `nostr://npub18lav8fkgt8424rxamvk8qq4xuy9n8mltjtgztv2w44hc5tt9vets0hcfsz/relay.ngit.dev/broadcast-relay`
- **Repository (Web):** https://gitworkshop.dev/girino@girino.org/broadcast-relay
- **License:** https://license.girino.org
- **Nostr NIPs:** https://github.com/nostr-protocol/nips
- **Khatru Framework:** https://github.com/fiatjaf/khatru

## Support

- **Issues:** [Report Issues](https://gitworkshop.dev/girino@girino.org/broadcast-relay/issues)
- **Discussions:** [Discussions](https://gitworkshop.dev/girino@girino.org/broadcast-relay/discussions)
- **Nostr:** Contact via relay's contact pubkey

## Acknowledgments

- Built with [khatru](https://github.com/fiatjaf/khatru) relay framework
- Uses [go-nostr](https://github.com/nbd-wtf/go-nostr) library
- Inspired by the Nostr community
- Icons and banners generated with Midjourney AI

---

<div align="center">

**Made with 💜 for the Nostr ecosystem**

[⭐ Star this repo](https://gitworkshop.dev/girino@girino.org/broadcast-relay) • [🐛 Report Bug](https://gitworkshop.dev/girino@girino.org/broadcast-relay/issues) • [💡 Request Feature](https://gitworkshop.dev/girino@girino.org/broadcast-relay/issues)

</div>
