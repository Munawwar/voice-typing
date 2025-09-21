.PHONY: build clean install test run release dist

# Build variables
BINARY_NAME=voice-typing
# Extract version from Go source code (e.g., VERSION = "0.1.0")
VERSION=$(shell grep 'VERSION.*=' main.go | cut -d'"' -f2)
BUILD_DIR=build
DIST_DIR=dist
CONFIG_FILE=config.json

# Build the binary
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	go build -trimpath -ldflags="-buildid=" -o $(BINARY_NAME) .
	@echo "✅ Build complete: ./$(BINARY_NAME)"

# Build for release with optimizations
release:
	@echo "🚀 Building release version..."
	CGO_ENABLED=1 go build -trimpath -ldflags="-w -s -buildid=" -o $(BINARY_NAME) .
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
	@echo "  dist           - Create distribution package"

# Create distribution package
dist: build
	@echo "📦 Creating distribution package..."
	@echo "Version: $(VERSION)"
	
	# Create dist directory
	mkdir -p $(DIST_DIR)
	
	# Create temporary directory for packaging
	$(eval TEMP_DIR := $(shell mktemp -d))
	$(eval PACKAGE_NAME := $(BINARY_NAME)-$(VERSION))
	mkdir -p $(TEMP_DIR)/$(PACKAGE_NAME)
	
	# Copy distribution files
	cp $(BINARY_NAME) $(TEMP_DIR)/$(PACKAGE_NAME)/
	cp install.sh $(TEMP_DIR)/$(PACKAGE_NAME)/
	cp uninstall.sh $(TEMP_DIR)/$(PACKAGE_NAME)/
	cp config.example.json $(TEMP_DIR)/$(PACKAGE_NAME)/
	cp README.md $(TEMP_DIR)/$(PACKAGE_NAME)/
	
	# Make scripts executable
	chmod +x $(TEMP_DIR)/$(PACKAGE_NAME)/install.sh
	chmod +x $(TEMP_DIR)/$(PACKAGE_NAME)/uninstall.sh
	chmod +x $(TEMP_DIR)/$(PACKAGE_NAME)/$(BINARY_NAME)
	
	# Create zip package
	cd $(TEMP_DIR) && zip -r $(CURDIR)/$(DIST_DIR)/$(PACKAGE_NAME).zip $(PACKAGE_NAME)
	
	# Clean up temp directory
	rm -rf $(TEMP_DIR)
	
	@echo "✅ Distribution package created: $(DIST_DIR)/$(PACKAGE_NAME).zip"
	@echo "📂 Contents: binary, install.sh, uninstall.sh, config.example.json, README.md"
