SCRIPT := diagnose.sh

.PHONY: run audit install help

## run: Check cluster-admin bindings
run:
	bash $(SCRIPT)

## audit: Full cluster RBAC audit (wildcard perms, bindings)
audit:
	bash $(SCRIPT) --cluster-audit

## install: Copy skill to ~/.claude-skills/skills/k8s-rbac-audit
install:
	mkdir -p $(HOME)/.claude-skills/skills/k8s-rbac-audit
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/k8s-rbac-audit/
	chmod +x $(HOME)/.claude-skills/skills/k8s-rbac-audit/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
