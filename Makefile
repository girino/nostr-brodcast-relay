.PHONY: build clean run test help

# Build the broadcast relay
build:
	@echo "Building broadcast-relay..."
	@rm -f broadcast-relay
	@go build -o broadcast-relay
	@echo "Build complete: ./broadcast-relay"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f broadcast-relay
	@go clean
	@echo "Clean complete"

# Run the relay (requires SEED_RELAYS environment variable)
run: build
	@echo "Starting broadcast-relay..."
	@./broadcast-relay

# Run tests
test:
	@echo "Running tests..."
	@go test ./...

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	@go vet ./...

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Display help
help:
	@echo "Broadcast Relay Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build    - Build the binary"
	@echo "  make clean    - Remove build artifacts"
	@echo "  make run      - Build and run (requires SEED_RELAYS env var)"
	@echo "  make test     - Run tests"
	@echo "  make fmt      - Format code"
	@echo "  make lint     - Lint code"
	@echo "  make deps     - Install/update dependencies"
	@echo "  make help     - Show this help message"
	@echo ""
	@echo "Example:"
	@echo "  export SEED_RELAYS='wss://relay.damus.io,wss://nos.lol'"
	@echo "  make run"

