#!/usr/bin/env bash
# openshift-debug — OpenShift cluster diagnostics
# Usage: diagnose.sh [--operators] [--scc] [-n project] [--olm] [--all]

set -euo pipefail

NAMESPACE=""
CHECK_OPS=false
CHECK_SCC=false
CHECK_OLM=false
ALL=false

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    --operators) CHECK_OPS=true; shift ;;
    --scc)       CHECK_SCC=true; shift ;;
    --olm)       CHECK_OLM=true; shift ;;
    --all)       ALL=true; CHECK_OPS=true; CHECK_OLM=true; shift ;;
    -n)          NAMESPACE="$2"; shift 2 ;;
    -h|--help)   echo "Usage: $0 [--operators] [--scc] [-n project] [--olm] [--all]"; exit 0 ;;
    *)           shift ;;
  esac
done

# Check if oc or kubectl is available
OC_CMD="kubectl"
if command -v oc &>/dev/null; then OC_CMD="oc"; fi

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║   OpenShift Debug — OCP Diagnostics      ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"
echo -e "  CLI: ${CYAN}$OC_CMD${RESET}"

# ── Cluster operators ─────────────────────────────────────────────────────────
if $CHECK_OPS; then
  banner "Cluster Operators"
  $OC_CMD get co 2>/dev/null | while read -r line; do
    if echo "$line" | grep -qE "False.*False.*True|False.*True"; then
      echo -e "${RED}  ✗ (DEGRADED) $line${RESET}"
    elif echo "$line" | grep -qE "True.*False.*False"; then
      echo -e "${GREEN}  ✓ $line${RESET}"
    else
      echo "  $line"
    fi
  done
fi

# ── Node health ───────────────────────────────────────────────────────────────
banner "Node Status"
$OC_CMD get nodes 2>/dev/null | while read -r line; do
  if echo "$line" | grep -q "NotReady\|SchedulingDisabled"; then
    echo -e "${RED}  ✗ $line${RESET}"
  elif echo "$line" | grep -q " Ready"; then
    echo -e "${GREEN}  ✓ $line${RESET}"
  else
    echo "  $line"
  fi
done

# ── MachineConfigPool ─────────────────────────────────────────────────────────
banner "MachineConfigPool Status"
$OC_CMD get mcp 2>/dev/null | while read -r line; do
  if echo "$line" | grep -qi "degraded\|updating"; then
    echo -e "${YELLOW}  ⚠ $line${RESET}"
  else
    echo "  $line"
  fi
done

# ── Routes ────────────────────────────────────────────────────────────────────
NS_FLAG="${NAMESPACE:+-n $NAMESPACE}"
if [ -z "$NAMESPACE" ]; then NS_FLAG="-A"; fi

banner "Routes"
$OC_CMD get routes $NS_FLAG 2>/dev/null | head -20

# ── OLM / Operator status ─────────────────────────────────────────────────────
if $CHECK_OLM; then
  banner "OLM Operator CSVs"
  $OC_CMD get csv $NS_FLAG 2>/dev/null | while read -r line; do
    if echo "$line" | grep -qi "failed\|install"; then
      echo -e "${RED}  ✗ $line${RESET}"
    elif echo "$line" | grep -qi "succeeded"; then
      echo -e "${GREEN}  ✓ $line${RESET}"
    else
      echo "  $line"
    fi
  done

  banner "OLM Subscriptions"
  $OC_CMD get subscription $NS_FLAG 2>/dev/null
fi

# ── SCC usage ────────────────────────────────────────────────────────────────
if $CHECK_SCC && [ -n "$NAMESPACE" ]; then
  banner "SCC Usage in project: $NAMESPACE"
  echo -e "${CYAN}Pods and their admitted SCC:${RESET}"
  $OC_CMD get pods -n "$NAMESPACE" \
    -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.annotations.openshift\.io/scc}{"\n"}{end}' 2>/dev/null
fi

# ── Unhealthy pods ────────────────────────────────────────────────────────────
banner "Unhealthy Pods"
$OC_CMD get pods $NS_FLAG --field-selector='status.phase!=Running,status.phase!=Succeeded' \
  --no-headers 2>/dev/null | head -20 | \
  while read -r line; do echo -e "${RED}  ✗ $line${RESET}"; done

echo -e "\n${GREEN}Key OpenShift commands:${RESET}"
echo "  oc get co                → cluster operators"
echo "  oc get mcp               → machine config pools"
echo "  oc adm must-gather       → collect diagnostics"
echo "  oc debug node/<name>     → shell into a node"
