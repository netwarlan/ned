BINARY := ned
MODULE := github.com/netwarlan/ned
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -w -s -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build test lint clean docker

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/ned

test:
	go test -v -race ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY)

docker:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(DATE) \
		-t ghcr.io/netwarlan/$(BINARY) .
