BINARY := askrelay
PKG := ./...
CMD := ./cmd/askrelay

.DEFAULT_GOAL := help

## help: show this help
.PHONY: help
help:
	@echo "askrelay — available make targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

## build: compile the askrelay binary
.PHONY: build
build:
	go build -o $(BINARY) $(CMD)

## run: build and run askrelay (pass args with ARGS="--help")
.PHONY: run
run:
	go run $(CMD) $(ARGS)

## test: run all tests with the race detector (needs cgo)
.PHONY: test
test:
	CGO_ENABLED=1 go test -race $(PKG)

## tidy: sync go.mod / go.sum
.PHONY: tidy
tidy:
	go mod tidy

## lint: run golangci-lint (uses .golangci.yml)
.PHONY: lint
lint:
	golangci-lint run

## overview: regenerate docs/askrelay-overview.html from the markdown docs
.PHONY: overview
overview:
	python3 docs/tools/build-overview.py
