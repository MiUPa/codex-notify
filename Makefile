APP := codex-notify

.PHONY: build test

build:
	go build -o bin/$(APP) .

test:
	go test ./...
