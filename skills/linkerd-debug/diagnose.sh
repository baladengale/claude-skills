#!/usr/bin/env bash
# linkerd-debug — Linkerd service mesh diagnostics
# Usage: diagnose.sh [--check] [-n ns] [--proxies] [--metrics] [--tap <deploy>]

set -euo pipefail

NAMESPACE=""
DO_CHECK=false
SHOW_PROXIES=false
SHOW_METRICS=false
TAP_DEPLOY=""

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    --check)    DO_CHECK=true; shift ;;
    -n)         NAMESPACE="$2"; shift 2 ;;
    --proxies)  SHOW_PROXIES=true; shift ;;
    --metrics)  SHOW_METRICS=true; shift ;;
    --tap)      TAP_DEPLOY="$2"; shift 2 ;;
    -h|--help)  echo "Usage: $0 [--check] [-n ns] [--proxies] [--metrics] [--tap deploy]"; exit 0 ;;
    *)          shift ;;
  esac
done

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║   Linkerd Debug — Mesh Diagnostics       ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"

# ── Check linkerd CLI ──────────────────────────────────────────────────────────
if ! command -v linkerd &>/dev/null; then
  echo -e "${YELLOW}  linkerd CLI not found.${RESET}"
  echo "  Install: curl -fsL https://run.linkerd.io/install | sh"
  echo "  Or: https://linkerd.io/2/getting-started/"
  exit 1
fi

# ── Control plane pods ────────────────────────────────────────────────────────
banner "Linkerd Control Plane"
kubectl get pods -n linkerd 2>/dev/null | while read -r line; do
  if echo "$line" | grep -qE "Error|CrashLoop|0/[0-9]"; then
    echo -e "${RED}  ✗ $line${RESET}"
  elif echo "$line" | grep -q "Running"; then
    echo -e "${GREEN}  ✓ $line${RESET}"
  else
    echo "  $line"
  fi
done

# ── Health check ──────────────────────────────────────────────────────────────
if $DO_CHECK; then
  banner "Linkerd Health Check"
  linkerd check 2>/dev/null | while read -r line; do
    if echo "$line" | grep -q "×"; then
      echo -e "${RED}$line${RESET}"
    elif echo "$line" | grep -q "‼"; then
      echo -e "${YELLOW}$line${RESET}"
    elif echo "$line" | grep -q "√"; then
      echo -e "${GREEN}$line${RESET}"
    else
      echo "$line"
    fi
  done
fi

# ── Proxy injection status ────────────────────────────────────────────────────
if $SHOW_PROXIES && [ -n "$NAMESPACE" ]; then
  banner "Proxy Injection (ns: $NAMESPACE)"
  echo -e "${CYAN}Namespace annotation:${RESET}"
  kubectl get namespace "$NAMESPACE" \
    -o jsonpath='{.metadata.annotations.linkerd\.io/inject}' 2>/dev/null
  echo ""
  echo -e "\n${CYAN}Pods and their containers:${RESET}"
  kubectl get pods -n "$NAMESPACE" \
    -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{range .spec.containers[*]}{.name}{" "}{end}{"\n"}{end}' 2>/dev/null | \
    while read -r line; do
      if echo "$line" | grep -q "linkerd-proxy"; then
        echo -e "${GREEN}  ✓ $line${RESET}"
      else
        echo -e "${YELLOW}  ⚠ (no proxy) $line${RESET}"
      fi
    done
fi

# ── Golden metrics ────────────────────────────────────────────────────────────
if $SHOW_METRICS && [ -n "$NAMESPACE" ]; then
  banner "Golden Metrics (ns: $NAMESPACE)"
  if command -v linkerd &>/dev/null; then
    linkerd viz stat deploy -n "$NAMESPACE" 2>/dev/null || \
      echo -e "${YELLOW}  linkerd viz not installed. Run: linkerd viz install | kubectl apply -f -${RESET}"
  fi
fi

# ── Tap ───────────────────────────────────────────────────────────────────────
if [ -n "$TAP_DEPLOY" ] && [ -n "$NAMESPACE" ]; then
  banner "Traffic Tap: $TAP_DEPLOY (5 seconds)"
  echo -e "${CYAN}  linkerd tap deployment/$TAP_DEPLOY -n $NAMESPACE${RESET}"
  timeout 5 linkerd tap "deployment/$TAP_DEPLOY" -n "$NAMESPACE" 2>/dev/null || \
    echo -e "${YELLOW}  Tap timeout or no traffic. Normal for idle services.${RESET}"
fi

# ── ServiceProfiles ───────────────────────────────────────────────────────────
if [ -n "$NAMESPACE" ]; then
  banner "ServiceProfiles (ns: $NAMESPACE)"
  kubectl get serviceprofiles -n "$NAMESPACE" 2>/dev/null || \
    echo -e "${YELLOW}  No ServiceProfiles found. Define routes for retries/timeouts/metrics.${RESET}"
fi

echo -e "\n${GREEN}Key commands:${RESET}"
echo "  linkerd check                          → full health check"
echo "  linkerd viz stat deploy -n <ns>        → golden metrics"
echo "  linkerd tap deploy/<name> -n <ns>      → live traffic"
echo "  linkerd viz dashboard                  → web UI"
