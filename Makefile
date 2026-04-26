BINARY := claude-ls
DIST   := dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build install clean release darwin-arm64 linux-amd64

all: build

build:
	go build -ldflags '$(LDFLAGS)' -o $(BINARY) .

install:
	go install -ldflags '$(LDFLAGS)' .

clean:
	rm -rf $(BINARY) $(DIST)

release: clean darwin-arm64 linux-amd64
	@ls -lh $(DIST)

darwin-arm64:
	@mkdir -p $(DIST)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 \
		go build -trimpath -ldflags '$(LDFLAGS)' \
		-o $(DIST)/$(BINARY)-darwin-arm64 .

linux-amd64:
	@mkdir -p $(DIST)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags '$(LDFLAGS)' \
		-o $(DIST)/$(BINARY)-linux-amd64 .
