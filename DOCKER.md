# Docker Deployment Guide

This guide explains how to deploy the broadcast-relay using Docker and Docker Compose with Tor hidden service support.

## Quick Start

### 1. Build the Docker Image

```bash
docker build -t broadcast-relay:latest .
```

### 2. Run with Docker Compose (Production)

```bash
# Start all services (relay + Tor)
docker-compose -f docker-compose.prod.yml up -d

# View logs
docker-compose -f docker-compose.prod.yml logs -f

# Get your Tor hidden service address
cat tor-data/hostname
```

## Configuration

### Environment Variables

All configuration is done through environment variables. See the docker-compose.prod.yml file for all available options:

- `RELAY_PORT`: WebSocket port (default: 3334)
- `SEED_RELAYS`: Comma-separated list of seed relays
- `MANDATORY_RELAYS`: Always broadcast to these relays
- `TOP_N_RELAYS`: Number of top relays to use (default: 50)
- `WORKER_COUNT`: Number of broadcast workers (default: 2×CPU)
- `CACHE_TTL`: Event cache duration (default: 5m)
- `REFRESH_INTERVAL`: Relay list refresh interval (default: 24h)
- `HEALTH_CHECK_INTERVAL`: Health check interval (default: 5m)

### Custom Configuration

Create a `.env` file in the project root:

```bash
# Relay Configuration
SEED_RELAYS=wss://relay.damus.io,wss://nos.lol,wss://relay.nostr.band
MANDATORY_RELAYS=wss://relay.example.com
TOP_N_RELAYS=100

# Performance
WORKER_COUNT=16
CACHE_TTL=10m
```

Then run:

```bash
docker-compose -f docker-compose.prod.yml up -d
```

## Services

### Relay Service

The main broadcast relay service. Exposed only to the internal Docker network by default.

**Access**: Internal only (through Tor or Nginx)

**Health Check**: Verifies `/stats` endpoint is responding

### Tor Hidden Service

Provides a .onion address for anonymous access.

**Health Check**: Verifies Tor process is running and hostname file exists

**Get your .onion address**:
```bash
cat tor-data/hostname
```

**Example output**: `abcd1234efgh5678.onion`

Your relay will be accessible at: `ws://abcd1234efgh5678.onion:80`

### Nginx Reverse Proxy (Optional)

For clearnet access with proper WebSocket support.

**Enable clearnet access**:
```bash
# Copy example config
cp nginx.conf.example nginx.conf

# Start with clearnet profile
docker-compose -f docker-compose.prod.yml --profile clearnet up -d
```

**Access**: `ws://localhost:3334` (or set custom port with `CLEARNET_PORT`)

**Health Check**: Verifies nginx can proxy to `/stats` endpoint

### Autoheal Service

Automatically monitors and restarts unhealthy containers.

**Features**:
- Monitors health checks every 10 seconds
- Automatically restarts containers that fail health checks
- Waits 30 seconds before monitoring new containers
- Monitors all services: relay, tor, and nginx (if enabled)

**View autoheal logs**:
```bash
docker-compose -f docker-compose.prod.yml logs -f autoheal
```

**Configuration**:
- `AUTOHEAL_INTERVAL=10` - Check health every 10 seconds
- `AUTOHEAL_START_PERIOD=30` - Grace period for new containers
- `AUTOHEAL_DEFAULT_STOP_TIMEOUT=10` - Wait time before force kill

## Tor Hidden Service

The Tor hidden service is automatically configured and started. Your relay will be accessible via Tor.

### Getting Your .onion Address

After starting the services:

```bash
# Wait a few seconds for Tor to generate keys
docker-compose -f docker-compose.prod.yml logs tor

# Get your address
cat tor-data/hostname
```

### Persistence

Tor keys are stored in `./tor-data/`. Keep this directory safe!

- **Backup**: Copy the entire `tor-data/` directory
- **Restore**: Place your backup in `tor-data/` before starting
- **New Address**: Delete `tor-data/` to generate a new .onion address

## Security

### Tor Data Protection

The following files are automatically excluded from git (`.gitignore`):

- `tor-data/` - Tor hidden service keys
- `hostname` - Your .onion address
- `private_key`, `hs_ed25519_secret_key` - Tor secret keys
- `nginx.conf` - May contain sensitive configuration

**⚠️ NEVER commit these files to version control!**

### Non-Root Execution

The relay runs as a non-root user (`relay:relay`, UID/GID 1000) inside the container for security.

## Monitoring

### Health Checks

All services have health checks configured:

- **Relay**: Checks `/stats` endpoint every 30s
- **Tor**: Verifies Tor process and hostname file every 30s  
- **Nginx**: Checks proxy to `/stats` endpoint every 30s
- **Autoheal**: Monitors all containers and auto-restarts unhealthy ones

```bash
# Check health status of all containers
docker-compose -f docker-compose.prod.yml ps

# View detailed health status
docker inspect broadcast-relay --format='{{.State.Health.Status}}'
docker inspect broadcast-relay-tor --format='{{.State.Health.Status}}'

# View stats endpoint
curl http://localhost:3334/stats
```

### Automatic Recovery

The autoheal service automatically restarts containers that fail health checks:

```bash
# View autoheal activity
docker-compose -f docker-compose.prod.yml logs -f autoheal

# Example autoheal log when container becomes unhealthy:
# [INFO] Container broadcast-relay is unhealthy, restarting...
```

### Logs

```bash
# All services
docker-compose -f docker-compose.prod.yml logs -f

# Specific service
docker-compose -f docker-compose.prod.yml logs -f relay
docker-compose -f docker-compose.prod.yml logs -f tor
```

### Stats Endpoint

The relay exposes a stats endpoint at `/stats`:

```bash
# Via clearnet (if nginx is enabled)
curl http://localhost:3334/stats

# Inside Docker network
docker exec broadcast-relay wget -qO- http://localhost:3334/stats
```

## Production Deployment

### Recommended Setup

1. **Set environment variables** in `.env` file
2. **Start services**:
   ```bash
   docker-compose -f docker-compose.prod.yml up -d
   ```
3. **Backup Tor keys**:
   ```bash
   tar -czf tor-backup.tar.gz tor-data/
   ```
4. **Share your .onion address**:
   ```bash
   echo "My relay: ws://$(cat tor-data/hostname):80"
   ```

### Updates

```bash
# Pull latest code
git pull

# Rebuild image
docker build -t broadcast-relay:latest .

# Restart services
docker-compose -f docker-compose.prod.yml up -d
```

### Maintenance

```bash
# View resource usage
docker stats broadcast-relay

# Restart relay only
docker-compose -f docker-compose.prod.yml restart relay

# Stop all services
docker-compose -f docker-compose.prod.yml down

# Stop and remove volumes (⚠️ loses Tor keys!)
docker-compose -f docker-compose.prod.yml down -v
```

## Troubleshooting

### Relay not starting

```bash
# Check logs
docker-compose -f docker-compose.prod.yml logs relay

# Check if port is in use
netstat -tulpn | grep 3334
```

### Tor not generating .onion address

```bash
# Check Tor logs
docker-compose -f docker-compose.prod.yml logs tor

# Restart Tor service
docker-compose -f docker-compose.prod.yml restart tor

# Wait 30 seconds, then check
cat tor-data/hostname
```

### Connection issues

```bash
# Test relay locally
wget -qO- http://localhost:3334/stats

# Test from inside container
docker exec broadcast-relay wget -qO- http://localhost:3334/stats
```

## Architecture

```
┌─────────────────┐
│   Tor Network   │
│  (your.onion)   │
└────────┬────────┘
         │
    ┌────▼─────┐  ◄────────────┐
    │   Tor    │               │
    │ Hidden   │  Health       │
    │ Service  │  Monitored    │
    └────┬─────┘               │    ┌──────────────┐
         │                     │    │  Autoheal    │
    ┌────▼─────┐  ◄────────────┤    │  Monitor     │
    │  Nginx   │◄──┐           │    │  & Restart   │
    │ (opt.)   │   │           │    └──────────────┘
    └────┬─────┘   │           │
         │         │           │
    ┌────▼─────────▼─┐  ◄──────┘
    │ Broadcast      │         ┌──────────────┐
    │ Relay          │◄────────│   Clearnet   │
    │ (port 3334)    │         │  (optional)  │
    └────────────────┘         └──────────────┘
         │
         │ Broadcasts to
         ▼
    ┌──────────────┐
    │ Nostr Relays │
    │ (Top N + Man)│
    └──────────────┘
```

## See Also

- [README.md](README.md) - General documentation
- [CONFIG.md](CONFIG.md) - Configuration options
- [VERBOSE_LOGGING.md](VERBOSE_LOGGING.md) - Debug logging guide

