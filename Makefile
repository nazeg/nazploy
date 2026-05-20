.PHONY: all build-frontend build-backend build clean run

# ── Configuration ──
BINARY_NAME  = dashboard
BUILD_DIR    = build
WEB_DIR      = web
GO_FLAGS     = -ldflags="-s -w" -trimpath

# ── Default ──
all: build

# ── Frontend ──
build-frontend:
	@echo "→ Building frontend..."
	cd $(WEB_DIR) && npm install && npm run build
	@echo "✓ Frontend built"

# ── Backend ──
build-backend: build-frontend
	@echo "→ Building Go backend..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(GO_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "✓ Backend built: $(BUILD_DIR)/$(BINARY_NAME)"

# ── All-in-one ──
build: build-backend
	@echo "✓ Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# ── Cross-compile for linux amd64 ──
build-linux: build-frontend
	@echo "→ Cross-compiling for linux/amd64..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(GO_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	@echo "✓ $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"

# ── Run locally (dev) ──
dev:
	@echo "→ Starting dev server on :8090..."
	go run .

# ── Clean ──
clean:
	rm -rf $(BUILD_DIR)
	rm -rf $(WEB_DIR)/dist
	rm -rf $(WEB_DIR)/node_modules
	@echo "✓ Cleaned"

# ── Install system deps ──
install-deps:
	@echo "→ Installing system dependencies..."
	apt-get update && apt-get install -y nginx certbot python3-certbot-nginx
	@echo "✓ System deps installed"
