SCRIPT := diagnose.sh

.PHONY: run list install help

## run: List all Helm releases with status
run:
	bash $(SCRIPT)

## list: Show all releases in all namespaces
list:
	bash $(SCRIPT)

## install: Copy skill to ~/.claude-skills/skills/helm-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/helm-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/helm-debug/
	chmod +x $(HOME)/.claude-skills/skills/helm-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
