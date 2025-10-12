# Granular Verbose Logging

The broadcast-relay now supports granular verbose logging by module and method, making it easier to debug specific parts of the system without flooding logs.

## Usage

### Enable All Verbose Logging
```bash
./broadcast-relay --verbose true
# or
./broadcast-relay -v true
```

### Enable Specific Modules
```bash
# Enable verbose logging for config and health modules only
./broadcast-relay --verbose "config,health"

# Enable broadcaster and manager modules
./broadcast-relay --verbose "broadcaster,manager"
```

### Enable Specific Methods
```bash
# Enable only the addEventToCache method in broadcaster
./broadcast-relay --verbose "broadcaster.addEventToCache"

# Enable multiple specific methods
./broadcast-relay --verbose "broadcaster.addEventToCache,health.CheckInitial"
```

### Mixed Mode
```bash
# Enable entire main module AND specific broadcaster method
./broadcast-relay --verbose "broadcaster.addEventToCache,main,config"
```

## Available Modules

- `config` - Configuration loading and parsing
- `health` - Health checking and relay testing
- `broadcaster` - Event broadcasting and caching
- `manager` - Relay management and scoring
- `discovery` - Relay discovery from seeds
- `relay` - HTTP/WebSocket relay server
- `main` - Main application logic

## Common Debug Scenarios

### Debug Event Caching Issues
```bash
./broadcast-relay --verbose "broadcaster.addEventToCache"
```

### Debug Configuration Loading
```bash
./broadcast-relay --verbose "config"
```

### Debug Relay Health Checks
```bash
./broadcast-relay --verbose "health"
```

### Debug Everything in Broadcaster
```bash
./broadcast-relay --verbose "broadcaster"
```

### Debug Multiple Systems
```bash
./broadcast-relay --verbose "config,health,broadcaster.addEventToCache"
```

## Log Output Format

Module-level logs:
```
[DEBUG] config: Loaded configuration: SeedRelays=1, MandatoryRelays=0, TopN=50, Port=3334, Workers=32
```

Method-level logs:
```
[DEBUG] broadcaster.addEventToCache: Adding event abc123... to cache (current size: 1523)
```

## Implementation

To add granular logging to your code:

```go
// Debug log with module and method
logging.DebugMethod("mymodule", "myMethod", "Processing %s", item)

// Always specify both module and method for better traceability
logging.DebugMethod("broadcaster", "addEventToCache", "Adding event %s to cache", eventID)
```

## Notes

- Regular `Info`, `Warn`, and `Error` logs are always shown
- Only `Debug` logs are affected by verbose filtering
- No performance impact when verbose is disabled
- Filters are case-sensitive

