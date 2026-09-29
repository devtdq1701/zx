.PHONY: all build test parity clean install

BINARY=bin/zx

all: build test

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY) ./cmd/zx

test:
	go test -v ./...

parity: build
	bash scripts/test_parity.sh

clean:
	rm -rf bin/ test_ram.png

install: build
	install -m 755 $(BINARY) $(HOME)/.local/bin/zx
