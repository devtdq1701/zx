VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -s -w -X zx/internal/cli.Version=$(VERSION)

.PHONY: all build test parity clean install

BINARY=bin/zx

all: build test

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/zx

test:
	go test -v ./...

parity: build
	bash scripts/test_parity.sh

clean:
	rm -rf bin/

install: build
	install -m 755 $(BINARY) $(HOME)/.local/bin/zx
