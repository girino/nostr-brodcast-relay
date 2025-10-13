# Docker Deployment Guide

This guide explains how to deploy the broadcast-relay using Docker and Docker Compose with Tor hidden service support.

## Quick Start

### 1. Build the Docker Image

```bash
docker build -t broadcast-relay:latest .
```

### 2. Run with Docker Compose (Production)

```bash
# Start all services (relay + Tor + autoheal)
docker-compose -f docker-compose.prod.yml up -d

# View logs
docker-compose -f docker-compose.prod.yml logs -f

# Get your Tor hidden service address (wait ~30s for Tor to bootstrap)
docker exec broadcast-relay-tor cat /var/lib/tor/hidden_service/relay/hostname
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

**Health Check**: 
- Verifies hostname file exists (hidden service created)
- Tests actual Tor connectivity through SOCKS proxy
- Queries Tor Project API to confirm traffic is routed through Tor
- More reliable than just checking if process is running

**Get your .onion address**:
```bash
# Using docker exec
docker exec broadcast-relay-tor cat /var/lib/tor/hidden_service/relay/hostname

# Or using docker-compose
docker-compose -f docker-compose.prod.yml exec tor cat /var/lib/tor/hidden_service/relay/hostname
```

**Example output**: `abcd1234efgh5678ijklmnop.onion`

Your relay will be accessible at: `ws://abcd1234efgh5678ijklmnop.onion:80`

### Nginx Reverse Proxy (Optional - Host Installation)

For clearnet access with proper WebSocket support. **Nginx runs on the host machine**, not in Docker.

The relay is exposed on `localhost:3334` from the Docker container, and nginx proxies to it.

**Setup nginx on host**:
```bash
# Install nginx (if not already installed)
sudo apt-get install nginx  # Ubuntu/Debian
# or
sudo yum install nginx      # CentOS/RHEL

# Copy the example config
sudo cp nginx.conf.example /etc/nginx/sites-available/broadcast-relay

# Edit to set your domain name
sudo nano /etc/nginx/sites-available/broadcast-relay

# Enable the site
sudo ln -s /etc/nginx/sites-available/broadcast-relay /etc/nginx/sites-enabled/

# Test configuration
sudo nginx -t

# Reload nginx
sudo systemctl reload nginx
```

**Access**: 
- HTTP: `ws://your-domain.com`
- HTTPS: Configure SSL (see commented section in nginx.conf.example)

**Note**: The relay container exposes port 3334 only to localhost (127.0.0.1:3334)

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

Tor keys are stored in a Docker named volume `tor-data`. Keep this safe!

**Backup**:
```bash
# Export volume to tar file
docker run --rm -v broadcast-relay_tor-data:/data -v $(pwd):/backup alpine tar czf /backup/tor-backup.tar.gz -C /data .
```

**Restore**:
```bash
# Import from tar file
docker run --rm -v broadcast-relay_tor-data:/data -v $(pwd):/backup alpine tar xzf /backup/tor-backup.tar.gz -C /data
```

**New Address**:
```bash
# Remove volume to generate new .onion address
docker-compose -f docker-compose.prod.yml down
docker volume rm broadcast-relay_tor-data
docker-compose -f docker-compose.prod.yml up -d
```

## Security

### Tor Data Protection

The Tor hidden service data is stored in a Docker named volume, not on the filesystem. This provides:
- Better security (isolated from host filesystem)
- Automatic permissions management
- No risk of accidentally committing sensitive files to git

**⚠️ Always backup your Tor volume** before updating or removing containers!

### Non-Root Execution

The relay runs as a non-root user (`relay:relay`, UID/GID 1000) inside the container for security.

## Monitoring

### Health Checks

All Docker services have health checks configured:

- **Relay**: Checks `/stats` endpoint every 30s
- **Tor**: Tests actual Tor connectivity every 60s
  - Verifies hidden service hostname file exists
  - Queries Tor Project API through SOCKS proxy
  - Confirms traffic is routed through Tor network
- **Autoheal**: Monitors all containers and auto-restarts unhealthy ones

**Note**: Nginx runs on the host and is monitored by systemd

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
3. **Get your .onion address** (wait ~60s for Tor bootstrap):
   ```bash
   docker exec broadcast-relay-tor cat /var/lib/tor/hidden_service/relay/hostname
   ```
4. **Backup Tor keys**:
   ```bash
   docker run --rm -v broadcast-relay_tor-data:/data -v $(pwd):/backup alpine tar czf /backup/tor-backup.tar.gz -C /data .
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
docker exec broadcast-relay-tor cat /var/lib/tor/hidden_service/relay/hostname
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
┌─────────────────────────────────────────────────────┐
│                    HOST MACHINE                     │
│                                                     │
│  ┌──────────────┐                                  │
│  │   Nginx      │◄─────────────┐                   │
│  │ (optional)   │              │                   │
│  └──────┬───────┘              │                   │
│         │                  Clearnet                │
│         │              (port 80/443)               │
│         │                                          │
│  ┌──────▼──────────────────────────────────────┐  │
│  │         DOCKER CONTAINERS                   │  │
│  │  ┌─────────────────────────────────┐        │  │
│  │  │  Broadcast Relay                │ ◄──┐   │  │
│  │  │  localhost:3334                 │    │   │  │
│  │  └──────────┬──────────────────────┘    │   │  │
│  │             │                            │   │  │
│  │       ┌─────▼──────┐   ┌─────────────┐  │   │  │
│  │       │    Tor     │   │  Autoheal   │──┘   │  │
│  │       │  Hidden    │   │  Monitor &  │      │  │
│  │       │  Service   │   │  Restart    │      │  │
│  │       └─────┬──────┘   └─────────────┘      │  │
│  │             │                                │  │
│  └─────────────┼────────────────────────────────┘  │
└────────────────┼───────────────────────────────────┘
                 │
           ┌─────▼──────┐
           │Tor Network │
           │(your.onion)│
           └────────────┘
                 │
                 │ Broadcasts to
                 ▼
           ┌─────────────┐
           │Nostr Relays │
           │(Top N + Man)│
           └─────────────┘
```

## See Also

- [README.md](README.md) - General documentation
- [CONFIG.md](CONFIG.md) - Configuration options
- [VERBOSE_LOGGING.md](VERBOSE_LOGGING.md) - Debug logging guide

