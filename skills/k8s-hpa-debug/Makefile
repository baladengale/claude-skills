SCRIPT := diagnose.sh

.PHONY: run metrics keda install help

## run: Check all HPA/VPA status
run:
	bash $(SCRIPT)

## metrics: Verify metrics-server
metrics:
	bash $(SCRIPT) --metrics-server

## keda: Check KEDA ScaledObjects
keda:
	bash $(SCRIPT) --keda

## install: Copy skill to ~/.claude-skills/skills/k8s-hpa-debug
install:
	mkdir -p $(HOME)/.claude-skills/skills/k8s-hpa-debug
	cp $(SCRIPT) SKILL.md Makefile $(HOME)/.claude-skills/skills/k8s-hpa-debug/
	chmod +x $(HOME)/.claude-skills/skills/k8s-hpa-debug/$(SCRIPT)

## help: Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
