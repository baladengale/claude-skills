#!/usr/bin/env bash
# tetrate-debug — Tetrate Service Bridge (TSB) diagnostics
# Usage: diagnose.sh [--health] [--sync] [--cluster] [--all]

set -euo pipefail

CHECK_HEALTH=false
CHECK_SYNC=false
CHECK_CLUSTER=false
ALL=false

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    --health)   CHECK_HEALTH=true; shift ;;
    --sync)     CHECK_SYNC=true; shift ;;
    --cluster)  CHECK_CLUSTER=true; shift ;;
    --all)      ALL=true; CHECK_HEALTH=true; CHECK_SYNC=true; CHECK_CLUSTER=true; shift ;;
    -h|--help)  echo "Usage: $0 [--health] [--sync] [--cluster] [--all]"; exit 0 ;;
    *)          shift ;;
  esac
done

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║   Tetrate Debug — TSB Diagnostics        ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"

# ── Check tctl ────────────────────────────────────────────────────────────────
if command -v tctl &>/dev/null; then
  echo -e "${GREEN}  tctl found: $(tctl version 2>/dev/null | head -1)${RESET}"
else
  echo -e "${YELLOW}  tctl not found. Install from: https://docs.tetrate.io/service-bridge/latest/reference/cli/guide/index${RESET}"
fi

# ── Management Plane health ───────────────────────────────────────────────────
if $CHECK_HEALTH; then
  banner "TSB Management Plane (namespace: tsb)"
  kubectl get pods -n tsb 2>/dev/null | while read -r line; do
    if echo "$line" | grep -qE "0/|Error|CrashLoop"; then
      echo -e "${RED}  ✗ $line${RESET}"
    elif echo "$line" | grep -q "Running"; then
      echo -e "${GREEN}  ✓ $line${RESET}"
    else
      echo "  $line"
    fi
  done
fi

# ── XCP Edge / control plane sync ────────────────────────────────────────────
if $CHECK_SYNC; then
  banner "XCP Edge (Control Plane Sync)"
  kubectl get pods -n istio-system 2>/dev/null | grep -i "xcp" | \
    while read -r line; do
      if echo "$line" | grep -qE "0/|Error|CrashLoop"; then
        echo -e "${RED}  ✗ $line${RESET}"
      elif echo "$line" | grep -q "Running"; then
        echo -e "${GREEN}  ✓ $line${RESET}"
      else
        echo "  $line"
      fi
    done

  echo -e "\n${CYAN}XCP Edge recent logs:${RESET}"
  XCP_POD=$(kubectl get pod -n istio-system -l app=xcp-edge \
    -o name 2>/dev/null | head -1)
  if [ -n "$XCP_POD" ]; then
    kubectl logs "$XCP_POD" -n istio-system --tail=20 2>/dev/null | \
      grep -iE "error|fail|disconnected|sync" | head -10
  else
    echo -e "${YELLOW}  No XCP Edge pod found in istio-system${RESET}"
  fi
fi

# ── Cluster listing via tctl ──────────────────────────────────────────────────
if $CHECK_CLUSTER && command -v tctl &>/dev/null; then
  banner "TSB Clusters (via tctl)"
  tctl get cluster 2>/dev/null | head -10 || \
    echo -e "${YELLOW}  Login required: tctl login --server <addr>:8443${RESET}"
fi

# ── Istio control plane (TSB manages Istiod) ──────────────────────────────────
banner "Istiod (TSB-managed)"
kubectl get pods -n istio-system 2>/dev/null | grep istiod | \
  while read -r line; do
    if echo "$line" | grep -q "Running"; then
      echo -e "${GREEN}  ✓ $line${RESET}"
    else
      echo -e "${RED}  ✗ $line${RESET}"
    fi
  done

echo -e "\n${GREEN}Key commands:${RESET}"
echo "  tctl login --server <addr>:8443   → authenticate"
echo "  tctl get cluster                  → list onboarded clusters"
echo "  tctl get workspace -t <tenant>    → list workspaces"
echo "  kubectl logs -n tsb deployment/xcp-central --tail=30  → MP sync logs"
echo "  kubectl logs -n istio-system deployment/xcp-edge --tail=30  → CP sync logs"
