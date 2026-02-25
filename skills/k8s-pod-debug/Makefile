SCRIPT := diagnose.sh

.PHONY: run check install help

## run: Run pod diagnostics (all namespaces)
run:
	bash $(SCRIPT) -a

## check: Run in current namespace
check:
	bash $(SCRIPT)

## install: Copy skill to ~/.claude-skills/skills/k8s-pod-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/k8s-pod-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/k8s-pod-debug/
	chmod +x $(HOME)/.claude-skills/skills/k8s-pod-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
