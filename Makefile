.PHONY: build clean run test help docker docker-up docker-down release-binaries

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

# Docker operations
docker:
	@echo "Building Docker image..."
	@docker build -t broadcast-relay:latest .

docker-up:
	@echo "Starting Docker services..."
	@docker-compose -f docker-compose.prod.yml up -d

docker-down:
	@echo "Stopping Docker services..."
	@docker-compose -f docker-compose.prod.yml down

docker-logs:
	@docker-compose -f docker-compose.prod.yml logs -f

# Release binaries for multiple platforms
release-binaries:
	@echo "Building release binaries..."
	@mkdir -p release
	@echo "Building Linux AMD64..."
	@GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o release/broadcast-relay-linux-amd64
	@echo "Building Linux ARM64..."
	@GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o release/broadcast-relay-linux-arm64
	@echo "Building macOS AMD64..."
	@GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o release/broadcast-relay-darwin-amd64
	@echo "Building macOS ARM64..."
	@GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o release/broadcast-relay-darwin-arm64
	@echo "Building Windows AMD64..."
	@GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o release/broadcast-relay-windows-amd64.exe
	@echo "Creating checksums..."
	@cd release && sha256sum * > checksums.txt
	@echo "Release binaries created in ./release/"

# Display help
help:
	@echo "Broadcast Relay Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build             - Build the binary"
	@echo "  make clean             - Remove build artifacts"
	@echo "  make run               - Build and run"
	@echo "  make test              - Run tests"
	@echo "  make fmt               - Format code"
	@echo "  make lint              - Lint code"
	@echo "  make deps              - Install/update dependencies"
	@echo "  make docker            - Build Docker image"
	@echo "  make docker-up         - Start Docker compose services"
	@echo "  make docker-down       - Stop Docker compose services"
	@echo "  make docker-logs       - View Docker logs"
	@echo "  make release-binaries  - Build binaries for all platforms"
	@echo "  make help              - Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make build && ./broadcast-relay --verbose all"
	@echo "  make docker-up"
	@echo "  make release-binaries"

