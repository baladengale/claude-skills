SCRIPT := diagnose.sh

.PHONY: run install help

## run: Check PV/PVC/StorageClass status
run:
	bash $(SCRIPT)

## install: Copy skill to ~/.claude-skills/skills/k8s-storage-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/k8s-storage-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/k8s-storage-debug/
	chmod +x $(HOME)/.claude-skills/skills/k8s-storage-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
