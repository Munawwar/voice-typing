.PHONY: build clean install test run release

# Build variables
BINARY_NAME=voice-typing
VERSION=1.0.0
BUILD_DIR=build
CONFIG_FILE=config.json

# Build the binary
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) .
	@echo "✅ Build complete: ./$(BINARY_NAME)"

# Build for release with optimizations
release:
	@echo "🚀 Building release version..."
	CGO_ENABLED=1 go build -ldflags="-w -s" -o $(BINARY_NAME) .
	@echo "✅ Release build complete: ./$(BINARY_NAME)"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)
	@echo "✅ Clean complete"

# Install dependencies
deps:
	@echo "📦 Installing dependencies..."
	go mod download
	go mod tidy
	@echo "✅ Dependencies installed"

# Run the application
run: build
	@if [ ! -f $(CONFIG_FILE) ]; then \
		echo "❌ Config file not found. Copy config.example.json to config.json and edit it."; \
		exit 1; \
	fi
	./$(BINARY_NAME)

# Run in hotkey mode
hotkey: build
	@if [ ! -f $(CONFIG_FILE) ]; then \
		echo "❌ Config file not found. Copy config.example.json to config.json and edit it."; \
		exit 1; \
	fi
	./$(BINARY_NAME) --hotkey

# Test the build
test:
	@echo "🧪 Running tests..."
	go test -v ./...


# Setup development environment
setup:
	@echo "🔧 Setting up development environment..."
	@if [ ! -f $(CONFIG_FILE) ]; then \
		echo "📝 Creating config file from example..."; \
		cp config.example.json $(CONFIG_FILE); \
		echo "⚠️  Please edit $(CONFIG_FILE) with your Deepgram API key"; \
	fi
	$(MAKE) deps
	@echo "✅ Setup complete"

# Check system requirements
check-deps:
	@echo "🔍 Checking system dependencies..."
	@command -v notify-send >/dev/null 2>&1 || echo "⚠️  notify-send not found (install libnotify-bin)"
	@command -v xdotool >/dev/null 2>&1 || echo "⚠️  xdotool not found (install xdotool)"
	@command -v wtype >/dev/null 2>&1 || echo "⚠️  wtype not found (install wtype for Wayland)"
	@command -v ydotool >/dev/null 2>&1 || echo "⚠️  ydotool not found (install ydotool for Wayland)"
	@systemctl --user is-active ydotoold >/dev/null 2>&1 || echo "⚠️  ydotoold service not running (systemctl --user enable --now ydotoold)"
	@echo "✅ Dependency check complete"

# Show help
help:
	@echo "Available commands:"
	@echo "  build          - Build the binary"
	@echo "  release        - Build optimized release version"
	@echo "  clean          - Clean build artifacts"
	@echo "  deps           - Install Go dependencies"
	@echo "  run            - Build and run the application"
	@echo "  hotkey         - Build and run in hotkey mode"
	@echo "  test           - Run tests"
	@echo "  setup          - Setup development environment"
	@echo "  check-deps     - Check system dependencies"
	@echo "  help           - Show this help message"
