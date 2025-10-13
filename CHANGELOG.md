# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2025-10-13

**First stable release of Broadcast Relay!** 🎉

A production-ready Nostr relay that broadcasts events to multiple relays with intelligent health monitoring and automatic relay discovery.

### Features
- **Worker Pool Architecture**: Configurable concurrent event broadcasting
- **Event Deduplication**: Time-based cache with configurable TTL (default 5 minutes)
- **Hybrid Queue System**: Bounded channel with unbounded overflow for reliability
- **Relay Discovery**: Automatic relay discovery from NIP-11 compatible relays
- **Health Monitoring**: Continuous relay health checks and performance tracking
- **Mandatory Relays**: Always-broadcast relay support with separate statistics
- **Granular Logging**: Module and method-specific verbose logging
- **Beautiful Web UI**: Landing page with relay information and branding
- **Multi-platform**: Linux, macOS, Windows on AMD64 and ARM64
- **Docker Support**: Production-ready Docker Compose with Tor hidden service
- **Automated Builds**: GitHub Actions for binary and Docker image releases

### Configuration
- 20+ environment variables for complete customization
- Sensible defaults (works out of the box)
- Auto-generated relay keypair if not configured
- Random banner selection from static assets

### Documentation
- Comprehensive README with examples and FAQ
- Complete configuration reference (CONFIG.md)
- Docker deployment guide with Tor (DOCKER.md)
- Contributor guidelines (CONTRIBUTING.md)
- Verbose logging guide (VERBOSE_LOGGING.md)

### Repository
- Primary: `nostr://npub18lav8fkgt8424rxamvk8qq4xuy9n8mltjtgztv2w44hc5tt9vets0hcfsz/relay.ngit.dev/broadcast-relay`
- Web: https://gitworkshop.dev/girino@girino.org/broadcast-relay
- License: Girino's Anarchist License (GAL)

## [1.0.0-rc2] - 2025-10-13

Second release candidate for v1.0.0 stable release.

### Fixed
- CI workflow test command simplified (removed covdata tool dependency)
- Docker workflow build attestation removed (permission issues)
- Dockerfile multi-arch support (removed hardcoded GOOS/GOARCH)
- GitHub Actions workflows now pass successfully

### Changed
- Contact pubkey defaults to relay pubkey if not configured
- Main page always shows relay and contact information

## [1.0.0-rc1] - 2025-10-13

First release candidate for v1.0.0 stable release.

All features from 0.2.0-rc1 plus:
- Complete documentation suite
- GitHub Actions for automated builds
- Nostr-based git repository
- Production-ready deployment guides

## [0.2.0-rc1] - 2025-10-13

### Added
- Worker pool architecture for event broadcasting
  - Configurable worker count via `WORKER_COUNT` env var
  - Defaults to 2× CPU cores if not configured
  - Event queue with worker processing

- Event deduplication cache
  - Prevents duplicate broadcasts
  - Time-based expiration (CACHE_TTL)
  - Size-limited (100K entries, ~10-13MB)
  - Automatic cleanup of expired entries
  - Cache statistics (hits, misses, hit rate)

- Hybrid queue architecture
  - Fixed-size channel (10× workers)
  - Unbounded overflow queue for burst traffic
  - Automatic backfill from overflow to channel
  - Queue saturation monitoring

- Granular verbose logging
  - Module-level filtering: `--verbose "config,health"`
  - Method-level filtering: `--verbose "broadcaster.addEventToCache"`
  - `--verbose all` for full debugging
  - DebugMethod() API for structured logging

- Mandatory relay tracking
  - Add relays that always receive events
  - Separate stats tracking
  - Health monitoring for mandatory relays
  - Shown separately in stats endpoint

- Relay metadata and main page
  - Beautiful HTML main page with relay information
  - NIP-11 compliant relay info
  - Configurable name, description, URL, contact
  - Auto-generated relay keypair if not configured
  - Icon and multiple banner support
  - Random banner selection per request

- Docker production setup
  - Multi-stage Dockerfile (~10MB final image)
  - docker-compose.prod.yml with Tor hidden service
  - Autoheal service for automatic recovery
  - Health checks for all services
  - nginx configuration example for host installation

- Static assets
  - Default icon (icon1.png)
  - 6 default banners with random selection
  - Static file server (/static/ endpoint)

### Changed
- Relay constructor now takes full config instead of individual parameters
- Contact pubkey defaults to relay pubkey if not configured
- Relay URL defaults to ws://localhost:{PORT} if not configured
- Improved Tor health check with connectivity testing
- Channel size reduced from 50× to 10× worker count
- Cache TTL default changed from 24h to 5 minutes
- Duplicate events now rejected at relay level (not silently accepted)

### Fixed
- Manager warning for unknown mandatory relays (now tracked properly)
- Tor hidden service configuration (correct environment variables)
- Docker build with latest Go version support
- HTML template moved to separate file (templates/main.html)

### Documentation
- Comprehensive README.md with all features
- CONTRIBUTING.md for contributors
- DOCKER.md for Docker deployment
- VERBOSE_LOGGING.md for debugging
- CONFIG.md for configuration reference
- nginx.conf.example for reverse proxy
- example.env with all configuration options

## [0.1.0-rc1] - 2025-10-13

### Added
- Initial release
- Basic relay broadcasting functionality
- Relay discovery from seeds
- Health checking
- Relay ranking and scoring
- Top N relay selection
- Stats endpoint
- Configurable via environment variables

---

## Versioning

We use [Semantic Versioning](https://semver.org/):
- **MAJOR**: Incompatible API changes
- **MINOR**: Backward-compatible new features  
- **PATCH**: Backward-compatible bug fixes
- **RC**: Release candidate (pre-release testing)

## Upgrade Notes

### From 0.1.0 to 0.2.0

**Breaking Changes:**
- Relay constructor signature changed (now takes `*config.Config`)

**New Environment Variables:**
- `WORKER_COUNT`: Broadcast worker configuration
- `CACHE_TTL`: Event cache duration
- `RELAY_NAME`, `RELAY_DESCRIPTION`, `RELAY_URL`: Relay metadata
- `CONTACT_PUBKEY`, `RELAY_PRIVKEY`: Identity configuration
- `RELAY_ICON`, `RELAY_BANNERS`: Branding assets

**Migration:**
- Update `relay.NewRelay()` calls to pass config instead of port
- Set new environment variables in your `.env` file
- Review and update Docker configuration if using containers

