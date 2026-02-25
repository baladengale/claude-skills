SCRIPT := diagnose.sh

.PHONY: run bgp ipam install help

## run: Calico health and IP pool check
run:
	bash $(SCRIPT) --health

## bgp: Check BGP peer status
bgp:
	bash $(SCRIPT) --bgp

## ipam: Show IP allocation blocks
ipam:
	bash $(SCRIPT) --ipam

## install: Copy skill to ~/.claude-skills/skills/calico-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/calico-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/calico-debug/
	chmod +x $(HOME)/.claude-skills/skills/calico-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
