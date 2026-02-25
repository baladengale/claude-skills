SCRIPT := diagnose.sh

.PHONY: run conntrack install help

## run: kube-proxy health check
run:
	bash $(SCRIPT)

## conntrack: Show conntrack table guidance
conntrack:
	bash $(SCRIPT) --conntrack

## install: Copy skill to ~/.claude-skills/skills/kube-proxy-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/kube-proxy-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/kube-proxy-debug/
	chmod +x $(HOME)/.claude-skills/skills/kube-proxy-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
