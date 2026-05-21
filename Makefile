GOCACHE ?= $(CURDIR)/.cache/go-build

.PHONY: test race build run fmt

fmt:
	gofmt -w cmd internal

test:
	GOCACHE=$(GOCACHE) go test ./...

race:
	GOCACHE=$(GOCACHE) go test -race ./...

build:
	GOCACHE=$(GOCACHE) go build -o bin/server ./cmd/server

run:
	GOCACHE=$(GOCACHE) go run ./cmd/server
