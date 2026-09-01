.PHONY: build test test-race test-integration lint lint-integration

build:
	go build -v ./...

test:
	go test -v ./...

test-race:
	go test -v -race ./...

# Requires a Chrome/Chromium binary on PATH.
test-integration:
	go test -v -race -tags=integration ./...

lint:
	go vet ./...
	golangci-lint run

lint-integration:
	go vet -tags=integration ./...
	golangci-lint run --build-tags=integration
