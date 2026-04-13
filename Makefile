.PHONY: build test lint
build:
	go build -o sonicforge ./cmd/sonicforge/

test:
	go test ./...

lint:
	golangci-lint run