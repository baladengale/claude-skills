SCRIPT := diagnose.sh

.PHONY: run resources install help

## run: Node health summary
run:
	bash $(SCRIPT)

## resources: Show resource allocation across all nodes
resources:
	bash $(SCRIPT) --resources

## install: Copy skill to ~/.claude-skills/skills/k8s-node-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/k8s-node-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/k8s-node-debug/
	chmod +x $(HOME)/.claude-skills/skills/k8s-node-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
