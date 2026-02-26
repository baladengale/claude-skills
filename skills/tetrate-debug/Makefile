SCRIPT := diagnose.sh

.PHONY: run all install help

## run: TSB management plane health
run:
	bash $(SCRIPT) --health --sync

## all: Full TSB diagnostics
all:
	bash $(SCRIPT) --all

## install: Copy skill to ~/.claude-skills/skills/tetrate-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/tetrate-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/tetrate-debug/
	chmod +x $(HOME)/.claude-skills/skills/tetrate-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
