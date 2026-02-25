SCRIPT := diagnose.sh

.PHONY: run cp install help

## run: Gateway API resource status
run:
	bash $(SCRIPT)

## cp: Check Envoy Gateway control plane
cp:
	bash $(SCRIPT) --control-plane

## install: Copy skill to ~/.claude-skills/skills/envoy-gateway-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/envoy-gateway-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/envoy-gateway-debug/
	chmod +x $(HOME)/.claude-skills/skills/envoy-gateway-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
