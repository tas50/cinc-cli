# cinc CLI build automation

BINARY      := cinc
CMD_PKG     := ./apps/cinc
LDFLAGS_PKG := github.com/tas50/cinc-cli/apps/cinc/cmd

# Build metadata — override on the command line if needed.
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE ?= $(shell date -u +%Y-%m-%d)

LDFLAGS := -X $(LDFLAGS_PKG).version=$(VERSION) \
           -X $(LDFLAGS_PKG).commit=$(COMMIT) \
           -X $(LDFLAGS_PKG).buildDate=$(BUILD_DATE)

.PHONY: all build install test test-acceptance vet fmt tidy clean run help

all: build

## build: compile the cinc binary with version metadata
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD_PKG)

## install: install cinc into the Go bin directory
install:
	go install -ldflags "$(LDFLAGS)" $(CMD_PKG)

## test: run the test suite
test:
	go test ./...

## test-acceptance: run acceptance tests against chef-zero (needs Ruby + the chef-zero gem)
test-acceptance:
	go test -tags acceptance -count=1 ./test/...

## vet: run go vet across all packages
vet:
	go vet ./...

## fmt: format all Go source
fmt:
	gofmt -w apps

## tidy: tidy go module dependencies
tidy:
	go mod tidy

## clean: remove build artifacts
clean:
	rm -f $(BINARY)
	go clean

## run: build and run cinc (pass flags via ARGS="...")
run: build
	./$(BINARY) $(ARGS)

## help: list available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //'
