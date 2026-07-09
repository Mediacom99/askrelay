BINARY := pchat
PKG := ./...
CMD := ./cmd/pchat

.DEFAULT_GOAL := help

## help: show this help
.PHONY: help
help:
	@echo "pchat — available make targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

## build: compile the pchat binary
.PHONY: build
build:
	go build -o $(BINARY) $(CMD)

## run: build and run pchat (pass args with ARGS="--room foo")
.PHONY: run
run:
	go run $(CMD) $(ARGS)

## test: run all tests
.PHONY: test
test:
	go test $(PKG)

## tidy: sync go.mod / go.sum
.PHONY: tidy
tidy:
	go mod tidy

## lint: run golangci-lint (uses .golangci.yml)
.PHONY: lint
lint:
	golangci-lint run

## overview: regenerate docs/pchat-overview.html from the markdown docs
.PHONY: overview
overview:
	python3 docs/tools/build-overview.py
