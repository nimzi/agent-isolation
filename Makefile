PREFIX ?= /usr/local
DESTDIR ?=

GO ?= go
BIN_NAME ?= ai-shell
PKG ?= ./cmd/ai-shell

.PHONY: build
build:
	install -d "bin"
	$(GO) build -o "bin/$(BIN_NAME)" $(PKG)

.PHONY: fmt
fmt:
	gofmt -w cmd internal

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: install
install:
	install -d "$(DESTDIR)$(PREFIX)/bin"
	$(GO) build -o "$(DESTDIR)$(PREFIX)/bin/$(BIN_NAME)" $(PKG)
	install -d "$(DESTDIR)$(PREFIX)/share/ai-shell"
	rm -rf "$(DESTDIR)$(PREFIX)/share/ai-shell/docker"
	cp -a docker "$(DESTDIR)$(PREFIX)/share/ai-shell/"

.PHONY: uninstall
uninstall:
	rm -f "$(DESTDIR)$(PREFIX)/bin/$(BIN_NAME)"
	rm -rf "$(DESTDIR)$(PREFIX)/share/ai-shell"

