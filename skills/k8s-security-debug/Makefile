SCRIPT := diagnose.sh

.PHONY: run posture gatekeeper falco install help

## run: Full security posture check
run:
	bash $(SCRIPT) --all

## posture: Check privileged pods and PSA labels
posture:
	bash $(SCRIPT) --posture

## gatekeeper: Check OPA Gatekeeper constraints
gatekeeper:
	bash $(SCRIPT) --gatekeeper

## falco: Check Falco alerts
falco:
	bash $(SCRIPT) --falco

## install: Copy skill to ~/.claude-skills/skills/k8s-security-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/k8s-security-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/k8s-security-debug/
	chmod +x $(HOME)/.claude-skills/skills/k8s-security-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
