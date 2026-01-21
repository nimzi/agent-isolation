PREFIX ?= /usr/local
DESTDIR ?=

GO ?= go
BIN_NAME ?= ai-shell
PKG ?= ./cmd/ai-shell

VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X github.com/nimzi/agent-isolation/internal/aishell.Version=$(VERSION)"

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

