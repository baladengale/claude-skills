SCRIPT := diagnose.sh

.PHONY: run dns policy install help

## run: Network diagnostics (current namespace)
run:
	bash $(SCRIPT)

## dns: Test DNS resolution
dns:
	bash $(SCRIPT) --dns

## policy: Audit NetworkPolicies
policy:
	bash $(SCRIPT) --policy

## install: Copy skill to ~/.claude-skills/skills/k8s-network-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/k8s-network-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/k8s-network-debug/
	chmod +x $(HOME)/.claude-skills/skills/k8s-network-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
