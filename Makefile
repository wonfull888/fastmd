VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X main.Version=$(VERSION)"

.PHONY: build build-server build-cli release clean

build: build-server build-cli

build-server:
	go build $(LDFLAGS) -o dist/fastmd-server ./cmd/server

build-cli:
	go build $(LDFLAGS) -o dist/fastmd ./cmd/cli

# Cross-compile all targets for release
release:
	mkdir -p dist
	GOOS=linux  GOARCH=amd64  go build $(LDFLAGS) -o dist/fastmd-linux-amd64   ./cmd/cli
	GOOS=linux  GOARCH=arm64  go build $(LDFLAGS) -o dist/fastmd-linux-arm64   ./cmd/cli
	GOOS=darwin GOARCH=amd64  go build $(LDFLAGS) -o dist/fastmd-darwin-amd64  ./cmd/cli
	GOOS=darwin GOARCH=arm64  go build $(LDFLAGS) -o dist/fastmd-darwin-arm64  ./cmd/cli
	GOOS=linux  GOARCH=amd64  go build $(LDFLAGS) -o dist/fastmd-server-linux-amd64  ./cmd/server
	GOOS=linux  GOARCH=arm64  go build $(LDFLAGS) -o dist/fastmd-server-linux-arm64  ./cmd/server

run:
	go run $(LDFLAGS) ./cmd/server --port 8080 --db /tmp/fastmd-dev.db

clean:
	rm -rf dist/
