# Contributing to Broadcast Relay

Thank you for your interest in contributing to Broadcast Relay! This document provides guidelines and information for contributors.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Release Process](#release-process)

## Code of Conduct

- **Be Respectful:** Treat everyone with respect and kindness
- **Be Constructive:** Provide helpful feedback and suggestions
- **Be Collaborative:** Work together to improve the project
- **Welcome Newcomers:** Help new contributors get started
- **Focus on Quality:** Maintain high code quality standards

## Getting Started

### Prerequisites

- Go 1.23 or higher
- Git
- Basic understanding of Nostr protocol
- Familiarity with Go programming

### Fork and Clone

```bash
# Fork the repository on GitHub, then:
git clone https://github.com/YOUR_USERNAME/nostr-brodcast-relay.git
cd nostr-brodcast-relay

# Add upstream remote
git remote add upstream https://github.com/girino/nostr-brodcast-relay.git

# Fetch latest changes
git fetch upstream
```

## Development Setup

### Install Dependencies

```bash
go mod download
```

### Build

```bash
go build -o broadcast-relay .
```

### Run Locally

```bash
# Basic run
./broadcast-relay

# With verbose logging
./broadcast-relay --verbose all

# With custom config
RELAY_PORT=8080 ./broadcast-relay --verbose "broadcaster,health"
```

## Making Changes

### Create a Feature Branch

```bash
git checkout -b feature/your-feature-name
```

**Branch naming conventions:**
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation changes
- `refactor/` - Code refactoring
- `perf/` - Performance improvements

### Make Your Changes

1. **Write Clear Code**
   - Follow existing patterns
   - Add comments for complex logic
   - Use meaningful variable names

2. **Use Proper Logging**
   ```go
   logging.DebugMethod("modulename", "methodname", "message %s", value)
   logging.Info("User-visible message")
   logging.Warn("Warning message")
   logging.Error("Error message")
   ```

3. **Handle Errors Properly**
   ```go
   if err != nil {
       logging.Error("Module: Operation failed: %v", err)
       return err
   }
   ```

4. **Use Thread-Safe Patterns**
   ```go
   // For shared state
   var mu sync.Mutex
   mu.Lock()
   defer mu.Unlock()
   
   // For counters
   atomic.AddInt64(&counter, 1)
   ```

## Coding Standards

### Go Style

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` for formatting (automatic)
- Run `go vet` to catch common issues

### Module Organization

```go
package modulename

import (
    "standard/library"
    
    "third/party"
    
    "github.com/girino/broadcast-relay/internal"
)

// Public functions are exported (capitalized)
func PublicFunction() {}

// Private functions are lowercase
func privateFunction() {}
```

### Logging Standards

- **DebugMethod**: Use for all debug logs
  ```go
  logging.DebugMethod("module", "method", "Processing event %s", id)
  ```

- **Info**: Important user-facing events
  ```go
  logging.Info("Relay: Starting %d workers", count)
  ```

- **Warn**: Recoverable issues
  ```go
  logging.Warn("Relay: Queue saturated, using overflow")
  ```

- **Error**: Serious errors
  ```go
  logging.Error("Relay: Failed to connect: %v", err)
  ```

### Configuration

- All configuration via environment variables
- Add to `config.Config` struct
- Use appropriate `getEnv*()` helper
- Document in `example.env`
- Provide sensible defaults

### Comments

```go
// PublicFunction does something important.
// It takes a parameter and returns a result.
// More detailed explanation if needed.
func PublicFunction(param string) result {
    // Complex logic deserves inline comments
    value := calculateSomething()
    
    return value
}
```

## Testing

### Manual Testing

```bash
# Build with race detector
go build -race -o broadcast-relay .

# Run with test relays
SEED_RELAYS="ws://localhost:10547" ./broadcast-relay --verbose all

# Monitor stats
watch -n 1 'curl -s http://localhost:3334/stats | jq'
```

### Testing Checklist

Before submitting:

- [ ] Code builds without errors
- [ ] No `go vet` warnings
- [ ] Tested with verbose logging
- [ ] Stats endpoint returns valid JSON
- [ ] Main page displays correctly
- [ ] WebSocket connections work
- [ ] Events are broadcast successfully
- [ ] No memory leaks (for long-running tests)
- [ ] Graceful shutdown works (Ctrl+C)

### Testing Specific Features

**Worker Pool:**
```bash
WORKER_COUNT=4 ./broadcast-relay --verbose "broadcaster.worker"
```

**Cache:**
```bash
CACHE_TTL=30s ./broadcast-relay --verbose "broadcaster.addEventToCache"
```

**Health Checks:**
```bash
./broadcast-relay --verbose "health.CheckInitial,health.TrackPublishResult"
```

## Submitting Changes

### Commit Messages

Use conventional commit format:

```
type(scope): subject

body (optional)

footer (optional)
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `style`: Formatting, no code change
- `refactor`: Code restructuring
- `perf`: Performance improvement
- `test`: Testing
- `chore`: Maintenance

**Examples:**
```
feat(broadcaster): add dynamic queue growth
fix(health): correct timeout handling
docs(readme): update installation instructions
perf(manager): optimize relay scoring algorithm
```

### Pull Request Process

1. **Update Documentation**
   - Update README.md if adding features
   - Update CONFIG.md for new config options
   - Add comments to code

2. **Commit Changes**
   ```bash
   git add .
   git commit -m "feat(module): description"
   ```

3. **Push to Your Fork**
   ```bash
   git push origin feature/your-feature-name
   ```

4. **Create Pull Request**
   - Go to GitHub repository
   - Click "New Pull Request"
   - Select your branch
   - Fill in template with:
     - What changed
     - Why it changed
     - How to test it
     - Related issues

5. **Respond to Review**
   - Address feedback promptly
   - Make requested changes
   - Push updates to same branch
   - Be open to suggestions

### Pull Request Checklist

- [ ] Code follows project style
- [ ] Comments added for complex code
- [ ] Documentation updated
- [ ] Tested manually
- [ ] No merge conflicts
- [ ] Commits are clean and logical
- [ ] PR description is clear

## Release Process

### Version Numbers

We use [Semantic Versioning](https://semver.org/):

- `MAJOR.MINOR.PATCH`
- `MAJOR`: Breaking changes
- `MINOR`: New features (backward compatible)
- `PATCH`: Bug fixes

**Examples:**
- `0.1.0` → `0.2.0`: New features added
- `0.2.0` → `0.2.1`: Bug fixes
- `0.2.1` → `1.0.0`: First stable release

### Release Checklist

1. **Update Version**
   - Update version in `relay/relay.go` (relay.Info.Version)
   - Update README.md current version

2. **Update Changelog**
   - List all changes since last release
   - Organize by type (Features, Fixes, etc.)
   - Credit contributors

3. **Test Thoroughly**
   - Build and run
   - Test all major features
   - Check Docker build
   - Verify stats endpoint
   - Test main page

4. **Create Release**
   ```bash
   git tag -a v0.X.0 -m "Release version 0.X.0"
   git push origin v0.X.0
   ```

5. **Build Binaries** (optional)
   ```bash
   # Linux AMD64
   GOOS=linux GOARCH=amd64 go build -o broadcast-relay-linux-amd64
   
   # Linux ARM64
   GOOS=linux GOARCH=arm64 go build -o broadcast-relay-linux-arm64
   
   # macOS AMD64
   GOOS=darwin GOARCH=amd64 go build -o broadcast-relay-darwin-amd64
   
   # macOS ARM64 (M1/M2)
   GOOS=darwin GOARCH=arm64 go build -o broadcast-relay-darwin-arm64
   
   # Windows
   GOOS=windows GOARCH=amd64 go build -o broadcast-relay-windows-amd64.exe
   ```

6. **Create GitHub Release**
   - Upload binaries
   - Add release notes
   - Mention breaking changes

## Development Tips

### Debugging

```bash
# Enable all debug logging
./broadcast-relay --verbose all

# Debug specific issues
./broadcast-relay --verbose "broadcaster.Broadcast,manager.UpdateHealth"

# Watch stats in real-time
watch -n 1 'curl -s http://localhost:3334/stats | jq ".queue, .cache"'
```

### Code Organization

- **One responsibility per module**
- **Clear interfaces between components**
- **Minimize global state**
- **Use dependency injection**

### Common Patterns

**Configuration:**
```go
// In config/config.go
type Config struct {
    NewSetting string
}

// In Load()
NewSetting: getEnv("NEW_SETTING", "default"),
```

**Logging:**
```go
logging.DebugMethod("module", "function", "Doing thing %s", value)
```

**Thread-Safe Updates:**
```go
atomic.AddInt64(&counter, 1)  // For counters
mu.Lock()                     // For complex state
defer mu.Unlock()
```

## Questions?

- Open an [issue](https://github.com/girino/nostr-brodcast-relay/issues)
- Start a [discussion](https://github.com/girino/nostr-brodcast-relay/discussions)
- Contact maintainer via Nostr

## Thank You!

Every contribution helps make Broadcast Relay better. Whether it's code, documentation, testing, or feedback - thank you for being part of this project! 💜

