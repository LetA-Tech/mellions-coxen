# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
SHELL := /bin/bash
.DEFAULT_GOAL := help

# One release version, read from the manifest every package agrees on.
VERSION ?= $(shell sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' .claude-plugin/plugin.json | head -1)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT)
SOURCE_DATE_EPOCH ?= $(shell git log -1 --format=%ct 2>/dev/null || echo 0)
DIST_DIR ?= dist

.PHONY: help
help:
	@echo "mellions — the second engineer"
	@echo
	@echo "  make build          bin/mellions for this machine"
	@echo "  make build-linux    CGO_ENABLED=0 linux/amd64 → bin/mellions-linux-amd64"
	@echo "  make release        dist/ tarballs for every supported platform"
	@echo "  make check          vet + race tests + the hooks and Skills"
	@echo "  make install        build, put the binary on PATH, register with the runtimes here"
	@echo "  make clean"

.PHONY: build
build:
	@mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/mellions ./cmd/mellions

.PHONY: build-linux
build-linux:
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/mellions-linux-amd64 ./cmd/mellions

# The platforms this runs on. The hooks are bash and the binary shells out to
# git and gh, so a Windows build would start and then not do the work.
PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64

.PHONY: release
release:
	@set -eu; \
	stage=$$(mktemp -d "$${TMPDIR:-/tmp}/mellions-release.XXXXXX"); \
	trap 'rm -rf "$$stage"' EXIT; \
	mkdir -p "$(DIST_DIR)"; \
	archives=""; \
	for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		out=$$stage/mellions_$(VERSION)_$${os}_$${arch}; \
		echo "  $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -trimpath -ldflags="$(LDFLAGS)" -o "$$out/mellions" ./cmd/mellions; \
		cp README.md LICENSE NOTICE "$$out/"; \
		mkdir -p "$$out/config"; \
		cp config/mellions.example.json "$$out/config/"; \
		go run ./cmd/releasepack -root "$$out" -output "$$out.tar.gz" -epoch "$(SOURCE_DATE_EPOCH)"; \
		name=$$(basename "$$out").tar.gz; \
		mv "$$out.tar.gz" "$(DIST_DIR)/$$name"; \
		archives="$$archives $$name"; \
	done; \
	(cd "$(DIST_DIR)" && shasum -a 256 $$archives > checksums.txt.tmp && mv checksums.txt.tmp checksums.txt); \
	ls -la "$(DIST_DIR)/"

.PHONY: test
test:
	# -count=1: the hook tests build this binary and run it through a script,
	# and Go's cache keys on source, not on the built artefact.
	go test -race -count=1 ./...

.PHONY: fmt-check
fmt-check:
	# go vet does not read formatting, so nothing else in check does either.
	# gofmt printing nothing is the pass, so a gofmt that did not run reads as
	# one: its absence and its errors both have to stop the target.
	@command -v gofmt >/dev/null || { echo "gofmt is not on PATH, so formatting was not checked"; exit 1; }
	@f=$$(gofmt -l .) || exit 1; if [ -n "$$f" ]; then \
		echo "not gofmt-ed:"; echo "$$f"; echo "run: gofmt -w $$f"; exit 1; \
	fi

.PHONY: vet
vet:
	go vet ./...

.PHONY: check-hooks
check-hooks:
	@for h in hooks/*.sh scripts/*.sh deploy/*.sh; do \
		[ -f "$$h" ] || continue; bash -n "$$h" || exit 1; \
	done
	@for t in hooks/test-*.sh scripts/test-*.sh deploy/test-*.sh; do \
		[ -f "$$t" ] || continue; \
		echo "  $$t"; \
		bash "$$t" || exit 1; \
	done
	@for t in skills/*/scripts/test-*.sh; do \
		[ -f "$$t" ] || continue; \
		echo "  $$t"; \
		bash "$$t" >/dev/null || { bash "$$t" | tail -20; exit 1; }; \
	done

.PHONY: check
check: fmt-check vet test check-hooks

# Install for this machine: the binary on PATH, then the plugin into whichever
# runtimes are here, from this checkout.
#
# Where the binary goes, and whether it landed somewhere a shell will run, is
# scripts/install-binary.sh — it prints the path it installed to. BIN= installs
# exactly there; PREFIX= is the first-install root when no mellions is on PATH.
PREFIX ?= /usr/local
BIN ?=
.PHONY: install
install: build
	@target=$$(SRC=bin/mellions PREFIX='$(PREFIX)' BIN='$(BIN)' scripts/install-binary.sh) && \
		"$$target" install -from .

.PHONY: clean
clean:
	rm -rf bin/ dist/
