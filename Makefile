PREFIX ?= /usr/local
DESTDIR ?=

GO ?= go
BIN_NAME ?= ai-shell
PKG ?= ./cmd/ai-shell

VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X github.com/nimzi/agent-isolation/internal/aishell.Version=$(VERSION)"

# Check for Go installation
GO_CHECK := $(shell command -v $(GO) 2>/dev/null)
ifndef GO_CHECK
$(error Go is not installed. Install it with:\
	- Ubuntu/Debian: sudo apt install golang-go\
	- Fedora: sudo dnf install golang\
	- macOS: brew install go\
	- Or download from https://go.dev/dl/)
endif

.PHONY: build
build:
	install -d "bin"
	$(GO) build $(LDFLAGS) -o "bin/$(BIN_NAME)" $(PKG)

.PHONY: fmt
fmt:
	gofmt -w cmd internal

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: install
install:
	install -d "$(DESTDIR)$(PREFIX)/bin"
	$(GO) build $(LDFLAGS) -o "$(DESTDIR)$(PREFIX)/bin/$(BIN_NAME)" $(PKG)

.PHONY: uninstall
uninstall:
	rm -f "$(DESTDIR)$(PREFIX)/bin/$(BIN_NAME)"

