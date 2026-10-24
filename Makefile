# nwm-panel build tooling.
#
# Release builds are produced from a checkout of a version tag (vX.Y.Z): the
# version and commit are taken from the tag. Any other checkout is a
# development build.

BINARY      := nwm-panel
MODULE      := github.com/tecncr/nwm-hri-modbus-panel
VERSION_PKG := $(MODULE)/internal/version
CMD         := ./cmd/nwm-panel
BUILD_DIR   := bin
WEB_DIR     := web

GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

TAG     := $(shell git describe --tags --exact-match --match 'v*' 2>/dev/null)
ifneq ($(TAG),)
VERSION := $(patsubst v%,%,$(TAG))
COMMIT  := $(shell git rev-parse HEAD)
else
VERSION := development build
COMMIT  := n/a
endif
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

VERSION_FLAGS := -X '$(VERSION_PKG).Version=$(VERSION)' \
                 -X '$(VERSION_PKG).Commit=$(COMMIT)' \
                 -X '$(VERSION_PKG).Date=$(DATE)' \
                 -X '$(VERSION_PKG).Os=$(GOOS)' \
                 -X '$(VERSION_PKG).Arch=$(GOARCH)'

GO_ENV := CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH)
OUT    := $(BUILD_DIR)/$(BINARY)$(if $(filter windows,$(GOOS)),.exe,)

PREFIX       ?= /usr/local
LOCAL_BINDIR ?= $(HOME)/.local/bin

.PHONY: all build build-debug web web-deps web-placeholder run dev \
        test test-timescale vet lint fmt check \
        install install-local install-global uninstall uninstall-local uninstall-global \
        clean clean-web distclean version help

all: build ## Build the dashboard and an optimized binary

build: web ## Build an optimized, stripped binary (CGO disabled)
	$(GO_ENV) go build -trimpath -ldflags "-s -w $(VERSION_FLAGS)" -o $(OUT) $(CMD)
	@echo "built $(OUT) ($(VERSION), $(GOOS)/$(GOARCH))"

build-debug: web ## Build an unoptimized binary with debug symbols
	$(GO_ENV) go build -gcflags "all=-N -l" -ldflags "$(VERSION_FLAGS)" -o $(OUT) $(CMD)
	@echo "built $(OUT) with debug information"

web-deps: ## Install dashboard dependencies
	cd $(WEB_DIR) && pnpm install --frozen-lockfile

web: web-deps ## Build the dashboard into web/dist (embedded in the binary)
	cd $(WEB_DIR) && pnpm build

# go:embed needs web/dist to exist; create a stub for Go-only workflows.
web-placeholder:
	@test -f $(WEB_DIR)/dist/index.html || { mkdir -p $(WEB_DIR)/dist && \
	  printf '<!doctype html><title>nwm-panel</title><p>Dashboard not built. Run <code>make web</code>.</p>\n' > $(WEB_DIR)/dist/index.html; }

run: build ## Build and run the server (requires NWM_PANEL_API_KEY)
	./$(OUT) serve

dev: web-placeholder ## Run the Go server from source, for use with `pnpm dev` in web/
	go run $(CMD) serve

test: web-placeholder ## Run Go tests with the race detector
	go test -race ./...

test-timescale: web-placeholder ## Also test against TimescaleDB (set NWM_PANEL_TEST_TIMESCALEDB_DSN)
	@test -n "$$NWM_PANEL_TEST_TIMESCALEDB_DSN" || { echo "set NWM_PANEL_TEST_TIMESCALEDB_DSN"; exit 1; }
	go test -race ./internal/store/...

vet: web-placeholder ## Run go vet
	go vet ./...

fmt: ## Format Go code
	gofmt -w cmd internal web/embed.go

lint: vet ## Run go vet and the dashboard type check and linter
	cd $(WEB_DIR) && pnpm exec tsc -b && pnpm lint

check: lint test ## Lint and test everything

install: install-local ## Alias for install-local

install-local: build ## Install to ~/.local/bin
	install -d $(LOCAL_BINDIR)
	install -m 0755 $(OUT) $(LOCAL_BINDIR)/$(BINARY)
	@echo "installed $(LOCAL_BINDIR)/$(BINARY)"

install-global: build ## Install to $(PREFIX)/bin (may need sudo)
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 0755 $(OUT) $(DESTDIR)$(PREFIX)/bin/$(BINARY)
	@echo "installed $(DESTDIR)$(PREFIX)/bin/$(BINARY)"

uninstall: uninstall-local ## Alias for uninstall-local

uninstall-local: ## Remove ~/.local/bin/nwm-panel
	rm -f $(LOCAL_BINDIR)/$(BINARY)

uninstall-global: ## Remove $(PREFIX)/bin/nwm-panel (may need sudo)
	rm -f $(DESTDIR)$(PREFIX)/bin/$(BINARY)

clean: ## Remove build output
	rm -rf $(BUILD_DIR)
	go clean -testcache

clean-web: ## Remove the built dashboard
	rm -rf $(WEB_DIR)/dist

distclean: clean clean-web ## Remove build output and dependencies
	rm -rf $(WEB_DIR)/node_modules

version: ## Print the version that would be embedded
	@echo "$(VERSION) (commit $(COMMIT), $(GOOS)/$(GOARCH))"

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-18s %s\n", $$1, $$2}'
