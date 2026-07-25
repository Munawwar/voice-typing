.PHONY: build release clean deps run hotkey stopkey test setup check-deps help dist

# Build variables
BINARY_NAME=voice-typing
VERSION=$(shell sed -n 's/^const version = "\(.*\)"/\1/p' main.go)
DIST_DIR=dist
DIST_PATH=$(abspath $(DIST_DIR))
CONFIG_FILE=config.json

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	go build -buildvcs=false -trimpath -ldflags="-buildid=" -o $(BINARY_NAME) .
	@echo "✅ Build complete: ./$(BINARY_NAME)"

# Build for release with optimizations
release:
	@echo "Building release version..."
	CGO_ENABLED=1 go build -buildvcs=false -trimpath -ldflags="-w -s -buildid=" -o $(BINARY_NAME) .
	@echo "✅ Release build complete: ./$(BINARY_NAME)"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	rm -rf dist
	@echo "✅ Clean complete"

# Tidy dependencies (rarely needed - go build downloads deps automatically)
deps:
	@echo "Tidying Go modules..."
	go mod tidy
	@echo "✅ Dependencies tidied"

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

# Run in stop hotkey mode
stopkey: build
	./$(BINARY_NAME) --stopkey

test:
	@echo "Running tests..."
	go test -v ./...


# Setup development environment
setup:
	@echo "Setting up development environment..."
	@if [ ! -f $(CONFIG_FILE) ]; then \
		echo "Creating config file from example..."; \
		cp config.example.json $(CONFIG_FILE); \
		echo "⚠️  Please edit $(CONFIG_FILE) with your Deepgram API key"; \
	fi
	$(MAKE) deps
	@echo "✅ Setup complete"

# Check system requirements
check-deps:
	@echo "Checking system dependencies..."
	@command -v notify-send >/dev/null 2>&1 || echo "⚠️  notify-send not found (install libnotify-bin)"
	@command -v xdotool >/dev/null 2>&1 || echo "⚠️  xdotool not found (install xdotool)"
	@command -v wtype >/dev/null 2>&1 || echo "⚠️  wtype not found (install wtype for Wayland)"
	@command -v ydotool >/dev/null 2>&1 || echo "⚠️  ydotool not found (install ydotool for Wayland)"
	@(systemctl --user is-active ydotoold >/dev/null 2>&1 || \
	  systemctl --user is-active ydotool >/dev/null 2>&1 || \
	  systemctl is-active ydotoold >/dev/null 2>&1 || \
	  systemctl is-active ydotool >/dev/null 2>&1) || \
	  echo "⚠️  ydotool daemon not running (try 'sudo systemctl enable --now ydotoold' or check a user unit)"
	@echo "✅ Dependency check complete"

# Show help
help:
	@echo "Available commands:"
	@echo "  build          - Build the binary"
	@echo "  release        - Build optimized release version"
	@echo "  clean          - Clean build artifacts"
	@echo "  deps           - Tidy Go dependencies"
	@echo "  run            - Build and run the application"
	@echo "  hotkey         - Build and run in hotkey mode"
	@echo "  stopkey        - Run stop hotkey command"
	@echo "  test           - Run tests"
	@echo "  setup          - Setup development environment"
	@echo "  check-deps     - Check system dependencies"
	@echo "  help           - Show this help message"
	@echo "  dist           - Create distribution package"

# Create distribution package
dist: build
	@echo "Creating distribution package..."
	@echo "Version: $(VERSION)"
	mkdir -p "$(DIST_PATH)"
	@set -eu; \
	package_name="$(BINARY_NAME)-$(VERSION)"; \
	package_tmp="$$(mktemp -d)"; \
	trap 'rm -rf "$$package_tmp"' EXIT; \
	mkdir -p "$$package_tmp/$$package_name"; \
	cp $(BINARY_NAME) install.sh uninstall.sh config.example.json README.md LICENSE "$$package_tmp/$$package_name/"; \
	chmod +x "$$package_tmp/$$package_name/install.sh" "$$package_tmp/$$package_name/uninstall.sh" "$$package_tmp/$$package_name/$(BINARY_NAME)"; \
	cd "$$package_tmp" && zip -r "$(DIST_PATH)/$$package_name.zip" "$$package_name"
	@echo "✅ Distribution package created: $(DIST_DIR)/$(BINARY_NAME)-$(VERSION).zip"
	@echo "Contents: binary, install.sh, uninstall.sh, config.example.json, README.md, LICENSE"
