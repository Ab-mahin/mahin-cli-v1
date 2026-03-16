BINARY  := mahin
MODULE  := github.com/mahin/mahin-cli-v1
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.0.1-dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    := $(shell date -u +%Y-%m-%d)

LDFLAGS := -ldflags "\
  -X $(MODULE)/version.Version=$(VERSION) \
  -X $(MODULE)/version.Commit=$(COMMIT) \
  -X $(MODULE)/version.BuildDate=$(DATE)"

.PHONY: all build run test clean install fmt vet lint tidy

## Default target: format, vet, then build
all: fmt vet build

## Build the binary
build:
	go build $(LDFLAGS) -o $(BINARY) .

## Run directly without building first
run:
	go run $(LDFLAGS) . $(ARGS)

## Run all tests
test:
	go test ./...

## Run tests with verbose output
test-v:
	go test -v ./...

## Install binary to ~/bin/
install: build
	@mkdir -p ~/bin
	cp $(BINARY) ~/bin/$(BINARY)
	@echo "Installed to ~/bin/$(BINARY)"

## Format source files
fmt:
	gofmt -l -w .

## Run go vet
vet:
	go vet ./...

## Tidy dependencies
tidy:
	go mod tidy

## Clean build artifacts
clean:
	rm -f $(BINARY)
	@echo "Cleaned."

## Show this help
help:
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' Makefile | sed 's/## /  /'
	@echo ""
