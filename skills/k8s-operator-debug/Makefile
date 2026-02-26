SCRIPT := diagnose.sh

.PHONY: run terminating webhooks install help

## run: List CRDs and operator health
run:
	bash $(SCRIPT) --all

## terminating: Find objects stuck in Terminating
terminating:
	bash $(SCRIPT) --terminating

## webhooks: Audit admission webhooks
webhooks:
	bash $(SCRIPT) --webhooks

## install: Copy skill to ~/.claude-skills/skills/k8s-operator-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/k8s-operator-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/k8s-operator-debug/
	chmod +x $(HOME)/.claude-skills/skills/k8s-operator-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
