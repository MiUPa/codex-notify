APP := codex-notify
TARGET_ARCH ?= $(shell uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')

.PHONY: build test

build:
	GOOS=darwin GOARCH=$(TARGET_ARCH) go build -o bin/$(APP) .

test:
	go test ./...
