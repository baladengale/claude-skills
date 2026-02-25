#!/usr/bin/env bash
# helm-debug — Helm release diagnostics and rollback helper
# Usage: diagnose.sh [--release <name>] [-n <ns>] [--history] [--rollback] [--diff]

set -euo pipefail

RELEASE=""
NAMESPACE=""
SHOW_HISTORY=false
DO_ROLLBACK=false
DO_DIFF=false

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    --release)   RELEASE="$2"; shift 2 ;;
    -n)          NAMESPACE="$2"; shift 2 ;;
    --history)   SHOW_HISTORY=true; shift ;;
    --rollback)  DO_ROLLBACK=true; shift ;;
    --diff)      DO_DIFF=true; shift ;;
    -h|--help)   echo "Usage: $0 [--release <name>] [-n <ns>] [--history] [--rollback]"; exit 0 ;;
    *)           shift ;;
  esac
done

NS_FLAG="${NAMESPACE:+-n $NAMESPACE}"
if [ -z "$NAMESPACE" ]; then NS_FLAG="-A"; fi

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║     Helm Debug — Release Diagnostics     ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"

# ── All releases ──────────────────────────────────────────────────────────────
banner "All Helm Releases"
helm list $NS_FLAG --output table 2>/dev/null | while read -r line; do
  if echo "$line" | grep -qE "failed|pending"; then
    echo -e "${RED}  $line${RESET}"
  elif echo "$line" | grep -q "deployed"; then
    echo -e "${GREEN}  $line${RESET}"
  else
    echo "  $line"
  fi
done

# ── Failed / Pending releases ─────────────────────────────────────────────────
banner "Failed / Pending Releases"
FAILED=$(helm list $NS_FLAG --failed --pending 2>/dev/null)
if [ -z "$FAILED" ] || [ "$(echo "$FAILED" | wc -l)" -le 1 ]; then
  echo -e "${GREEN}  No failed or pending releases.${RESET}"
else
  echo -e "${RED}$FAILED${RESET}"
fi

# ── Release detail ────────────────────────────────────────────────────────────
if [ -n "$RELEASE" ]; then
  NS_ARG="${NAMESPACE:+-n $NAMESPACE}"

  banner "Release Status: $RELEASE"
  helm status "$RELEASE" $NS_ARG 2>/dev/null || \
    echo -e "${RED}  Release not found: $RELEASE${RESET}"

  if $SHOW_HISTORY; then
    banner "Release History: $RELEASE"
    helm history "$RELEASE" $NS_ARG 2>/dev/null
  fi

  if $DO_ROLLBACK; then
    banner "Rolling Back: $RELEASE"
    echo -e "${YELLOW}  Running: helm rollback $RELEASE $NS_ARG${RESET}"
    helm rollback "$RELEASE" $NS_ARG 2>&1
    echo -e "${GREEN}  Rollback complete. Check: helm history $RELEASE $NS_ARG${RESET}"
  fi

  if $DO_DIFF; then
    if helm plugin list 2>/dev/null | grep -q "diff"; then
      banner "Release Diff: $RELEASE"
      echo -e "${CYAN}  Run: helm diff upgrade $RELEASE <chart> $NS_ARG -f values.yaml${RESET}"
    else
      warn "helm-diff plugin not installed. Install with:"
      echo "  helm plugin install https://github.com/databus23/helm-diff"
    fi
  fi

  # Show recent k8s events for release resources
  if [ -n "$NAMESPACE" ]; then
    banner "Recent Events (ns: $NAMESPACE)"
    kubectl get events -n "$NAMESPACE" \
      --sort-by='.lastTimestamp' 2>/dev/null | tail -15 | \
      while read -r line; do
        if echo "$line" | grep -qi "warning\|error\|failed"; then
          echo -e "${YELLOW}  $line${RESET}"
        else
          echo "  $line"
        fi
      done
  fi
fi

echo -e "\n${GREEN}Tips:${RESET}"
echo "  helm diff upgrade   — preview changes before upgrading (requires helm-diff plugin)"
echo "  helm history        — view all revisions"
echo "  helm rollback       — revert to previous revision"
echo "  helm upgrade --atomic --cleanup-on-fail  — safe upgrade with auto-rollback"
