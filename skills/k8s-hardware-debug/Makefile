SCRIPT := diagnose.sh

.PHONY: run gpu hugepages install help

## run: Full hardware resource audit
run:
	bash $(SCRIPT) --all

## gpu: GPU node inventory and health
gpu:
	bash $(SCRIPT) --gpu

## hugepages: Hugepages configuration
hugepages:
	bash $(SCRIPT) --hugepages

## install: Copy skill to ~/.claude-skills/skills/k8s-hardware-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/k8s-hardware-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/k8s-hardware-debug/
	chmod +x $(HOME)/.claude-skills/skills/k8s-hardware-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
