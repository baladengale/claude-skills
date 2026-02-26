SCRIPT := diagnose.sh

.PHONY: run analyze proxy-status install help

## run: Check control plane and traffic config
run:
	bash $(SCRIPT) --control-plane --analyze

## proxy-status: Check xDS proxy sync
proxy-status:
	bash $(SCRIPT) --proxy-status

## analyze: Run istioctl analyze across all namespaces
analyze:
	bash $(SCRIPT) --analyze

## install: Copy skill to ~/.claude-skills/skills/istio-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/istio-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/istio-debug/
	chmod +x $(HOME)/.claude-skills/skills/istio-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
