BINARY_NAME := azw3-to-pdf
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: build run test lint fmt ci install tidy clean snapshot release-check

build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/azw3-to-pdf/

run: build
	./$(BINARY_NAME)

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

ci: lint test build

install:
	go install $(LDFLAGS) ./cmd/azw3-to-pdf/

tidy:
	go mod tidy

clean:
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME).exe
	rm -rf dist/

snapshot:
	goreleaser build --snapshot --clean

release-check:
	goreleaser check
