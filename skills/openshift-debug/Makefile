SCRIPT := diagnose.sh

.PHONY: run operators olm install help

## run: Overall OCP cluster health
run:
	bash $(SCRIPT) --operators

## olm: Check OLM operator status
olm:
	bash $(SCRIPT) --olm

## install: Copy skill to ~/.claude-skills/skills/openshift-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/openshift-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/openshift-debug/
	chmod +x $(HOME)/.claude-skills/skills/openshift-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
