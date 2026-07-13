# Mandelbrot Set Explorer — Makefile
# =============================================================================

APP_NAME   := mandelbrot
GO         := go
GOFLAGS    := -v
LDFLAGS    := -s -w
BUILD_DIR  := build

# Default target.
.PHONY: all
all: build

# ---------------------------------------------------------------------------
# Build (desktop GUI — requires X11 dev headers)
# ---------------------------------------------------------------------------

.PHONY: build
build: $(BUILD_DIR)/$(APP_NAME)

$(BUILD_DIR)/$(APP_NAME): go.mod go.sum main.go mandelbrot/*.go
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ .
	@echo "√ $(APP_NAME) ($$(du -sh $@ | cut -f1))"

# ---------------------------------------------------------------------------
# Run
# ---------------------------------------------------------------------------

.PHONY: run
run: build
	cd $(BUILD_DIR) && ./$(APP_NAME)

# ---------------------------------------------------------------------------
# Test
# ---------------------------------------------------------------------------

.PHONY: test
test:
	$(GO) test $(GOFLAGS) -count=1 -race ./mandelbrot/...

.PHONY: test-cover
test-cover:
	$(GO) test $(GOFLAGS) -count=1 -race -coverprofile=coverage.out ./mandelbrot/...
	$(GO) tool cover -func=coverage.out
	@echo ""
	@echo "√ coverage report saved to coverage.out"

.PHONY: bench
bench:
	$(GO) test -bench=. -benchmem ./mandelbrot/...

# ---------------------------------------------------------------------------
# Clean
# ---------------------------------------------------------------------------

.PHONY: clean
clean:
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out
	@echo "√ cleaned"

# ---------------------------------------------------------------------------
# Help
# ---------------------------------------------------------------------------

.PHONY: help
help:
	@echo "Mandelbrot Set Explorer"
	@echo ""
	@echo "Usage:"
	@echo "  make              Build the desktop GUI binary"
	@echo "  make build        Same as above"
	@echo "  make run          Build and launch in fullscreen"
	@echo "  make test         Run unit tests with race detector"
	@echo "  make test-cover   Run tests with coverage report"
	@echo "  make bench        Run benchmarks"
	@echo "  make clean        Remove all build artifacts"
	@echo ""
	@echo "Controls:"
	@echo "  Mouse wheel       Zoom in / out at cursor position"
	@echo "  ↑ / ↓ / + / -     Increase / decrease iteration precision"
	@echo "  ← / →             Switch color palette"
	@echo "  R                 Reset view"
	@echo "  Esc               Quit"
	@echo ""
	@echo "Flags:"
	@echo "  -iter N           Max iterations (1–10000, default 256)"
	@echo "  -palette S        Palette: PICO-8, Fire, BlueFire, Twilight"
	@echo "  -rows N           Tile grid rows (0 = auto)"
	@echo "  -cols N           Tile grid cols (0 = auto)"
