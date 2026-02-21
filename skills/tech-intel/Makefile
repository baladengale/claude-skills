BINARY    := tech-intel
GOFLAGS   := -ldflags="-s -w"

.PHONY: build run clean fmt install linux darwin all help

## build: Compile for current OS/arch
build:
	go build $(GOFLAGS) -o $(BINARY) main.go

## run: Build and execute
run: build
	./$(BINARY)

## linux: Cross-compile for Linux amd64
linux:
	GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -o $(BINARY)-linux-amd64 main.go

## darwin: Cross-compile for macOS arm64
darwin:
	GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) -o $(BINARY)-darwin-arm64 main.go

## all: Build for both platforms
all: linux darwin

## install: Build and copy to ~/.claude-skills/skills/tech-intel
install: build
	mkdir -p $(HOME)/.claude-skills/skills/tech-intel
	cp $(BINARY) SKILL.md template.html go.mod Makefile main.go \
	   $(HOME)/.claude-skills/skills/tech-intel/

## fmt: Format source code
fmt:
	gofmt -w main.go

## clean: Remove build artifacts
clean:
	rm -f $(BINARY) $(BINARY)-linux-* $(BINARY)-darwin-*

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
