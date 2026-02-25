SCRIPT := diagnose.sh

.PHONY: run certs lb install help

## run: Check all ingress resources and LB status
run:
	bash $(SCRIPT)

## certs: Check cert-manager certificate status
certs:
	bash $(SCRIPT) --certs

## lb: Check LoadBalancer service IPs
lb:
	bash $(SCRIPT) --lb

## install: Copy skill to ~/.claude-skills/skills/k8s-ingress-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/k8s-ingress-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/k8s-ingress-debug/
	chmod +x $(HOME)/.claude-skills/skills/k8s-ingress-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
