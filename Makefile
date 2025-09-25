.PHONY: run test build fmt lint mod clean check

APP := github.com/athom/hotel-merge
BIN := hotel-merge

run:
	go run ./cmd/server

build:
	go build -o bin/$(BIN) ./cmd/server

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

lint:
	go vet ./...

mod:
	go mod tidy

clean:
	rm -rf bin

check: fmt lint test

test:
	go test ./...
