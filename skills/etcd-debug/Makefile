SCRIPT := diagnose.sh

.PHONY: run health perf install help

## run: etcd health check and cluster status
run:
	bash $(SCRIPT) --health

## health: Full health + alarm check
health:
	bash $(SCRIPT) --health --perf

## install: Copy skill to ~/.claude-skills/skills/etcd-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/etcd-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/etcd-debug/
	chmod +x $(HOME)/.claude-skills/skills/etcd-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
