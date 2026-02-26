SCRIPT := diagnose.sh

.PHONY: run images install help

## run: Runtime health and non-running containers
run:
	bash $(SCRIPT)

## images: List cached images and disk usage
images:
	bash $(SCRIPT) --images

## install: Copy skill to ~/.claude-skills/skills/container-runtime-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/container-runtime-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/container-runtime-debug/
	chmod +x $(HOME)/.claude-skills/skills/container-runtime-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
