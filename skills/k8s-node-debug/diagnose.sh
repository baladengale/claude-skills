#!/usr/bin/env bash
# k8s-node-debug — Node health diagnostics
# Usage: diagnose.sh [--node <name>] [--resources] [--drain <node>]

set -euo pipefail

NODE=""
SHOW_RESOURCES=false
DRAIN_NODE=""

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    --node)      NODE="$2"; shift 2 ;;
    --resources) SHOW_RESOURCES=true; shift ;;
    --drain)     DRAIN_NODE="$2"; shift 2 ;;
    -h|--help)   echo "Usage: $0 [--node <name>] [--resources]"; exit 0 ;;
    *)           shift ;;
  esac
done

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║    K8s Node Debug — Node Diagnostics     ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"

# ── Node Status ───────────────────────────────────────────────────────────────
banner "Node Status"
kubectl get nodes -o wide 2>/dev/null | while read -r line; do
  if echo "$line" | grep -q "NotReady\|SchedulingDisabled"; then
    echo -e "${RED}  ✗ $line${RESET}"
  elif echo "$line" | grep -q "Ready"; then
    echo -e "${GREEN}  ✓ $line${RESET}"
  else
    echo "  $line"
  fi
done

# ── Node Conditions ───────────────────────────────────────────────────────────
banner "Node Conditions"
kubectl get nodes -o json 2>/dev/null | \
  jq -r '.items[] | .metadata.name as $n | .status.conditions[] |
    select(.type != "Ready" and .status == "True") |
    "  ⚠ Node: " + $n + "  Condition: " + .type + "  Message: " + .message' \
  2>/dev/null | while read -r line; do echo -e "${YELLOW}$line${RESET}"; done || true

# ── Deep-dive on specific node ────────────────────────────────────────────────
if [ -n "$NODE" ]; then
  banner "Node Deep Dive: $NODE"
  echo -e "${CYAN}Conditions:${RESET}"
  kubectl describe node "$NODE" 2>/dev/null | grep -A30 "^Conditions:"

  echo -e "\n${CYAN}Allocated Resources:${RESET}"
  kubectl describe node "$NODE" 2>/dev/null | grep -A15 "Allocated resources:"

  echo -e "\n${CYAN}Recent Events:${RESET}"
  kubectl get events --all-namespaces \
    --field-selector="involvedObject.name=$NODE" \
    --sort-by='.lastTimestamp' 2>/dev/null | tail -15
fi

# ── Resource allocation ───────────────────────────────────────────────────────
if $SHOW_RESOURCES; then
  banner "Resource Allocation (all nodes)"
  kubectl describe nodes 2>/dev/null | \
    grep -E "^Name:|Allocatable|Allocated|cpu|memory" | \
    grep -v "Capacity" | head -60
fi

# ── Drain preview ─────────────────────────────────────────────────────────────
if [ -n "$DRAIN_NODE" ]; then
  banner "Drain Preview (dry-run): $DRAIN_NODE"
  echo -e "${YELLOW}  Pods that would be evicted:${RESET}"
  kubectl drain "$DRAIN_NODE" \
    --ignore-daemonsets \
    --delete-emptydir-data \
    --dry-run 2>/dev/null
fi

# ── Evicted pods cleanup ──────────────────────────────────────────────────────
banner "Evicted Pods (pending cleanup)"
EVICTED=$(kubectl get pods -A --field-selector=status.phase=Failed \
  --no-headers 2>/dev/null | grep -c "Evicted" || true)
if [ "$EVICTED" -gt 0 ]; then
  echo -e "${YELLOW}  $EVICTED evicted pods found. Clean up with:${RESET}"
  echo "  kubectl get pods -A --field-selector=status.phase=Failed | grep Evicted | awk '{print \$1, \$2}' | xargs -n2 kubectl delete pod -n"
else
  echo -e "${GREEN}  No evicted pods found.${RESET}"
fi

echo -e "\n${GREEN}Done. Use --node <name> for deep-dive or --resources for allocation summary.${RESET}"
