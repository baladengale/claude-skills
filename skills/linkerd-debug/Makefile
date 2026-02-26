SCRIPT := diagnose.sh

.PHONY: run check install help

## run: Show Linkerd control plane status
run:
	bash $(SCRIPT)

## check: Full Linkerd health check
check:
	bash $(SCRIPT) --check

## install: Copy skill to ~/.claude-skills/skills/linkerd-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/linkerd-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/linkerd-debug/
	chmod +x $(HOME)/.claude-skills/skills/linkerd-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
