#!/usr/bin/env bash
# calico-debug — Calico CNI diagnostics
# Usage: diagnose.sh [--health] [--bgp] [--ipam] [--pod <name> -n <ns>]

set -euo pipefail

DO_HEALTH=false
DO_BGP=false
DO_IPAM=false
POD=""
NAMESPACE=""

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    --health)  DO_HEALTH=true; shift ;;
    --bgp)     DO_BGP=true; shift ;;
    --ipam)    DO_IPAM=true; shift ;;
    --pod)     POD="$2"; shift 2 ;;
    -n)        NAMESPACE="$2"; shift 2 ;;
    -h|--help) echo "Usage: $0 [--health] [--bgp] [--ipam] [--pod <name> -n <ns>]"; exit 0 ;;
    *)         shift ;;
  esac
done

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║    Calico Debug — CNI Diagnostics        ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"

# ── Find a calico-node pod ────────────────────────────────────────────────────
CALICO_POD=$(kubectl get pod -n kube-system -l k8s-app=calico-node \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

# ── calico-node pod health ────────────────────────────────────────────────────
banner "calico-node DaemonSet Health"
kubectl get pods -n kube-system -l k8s-app=calico-node -o wide 2>/dev/null | \
  while read -r line; do
    if echo "$line" | grep -qE "0/|Error|CrashLoop"; then
      echo -e "${RED}  ✗ $line${RESET}"
    elif echo "$line" | grep -q "Running"; then
      echo -e "${GREEN}  ✓ $line${RESET}"
    else
      echo "  $line"
    fi
  done

# ── calico-kube-controllers ───────────────────────────────────────────────────
banner "calico-kube-controllers"
kubectl get pods -n kube-system -l k8s-app=calico-kube-controllers 2>/dev/null

# ── IP Pools ─────────────────────────────────────────────────────────────────
banner "IP Pools"
if command -v calicoctl &>/dev/null; then
  calicoctl get ippool -o wide 2>/dev/null
elif [ -n "$CALICO_POD" ]; then
  kubectl exec -n kube-system "$CALICO_POD" -- calicoctl get ippool -o wide 2>/dev/null || \
    echo -e "${YELLOW}  calicoctl not available in pod, check DATASTORE_TYPE${RESET}"
else
  echo -e "${YELLOW}  calicoctl not found. Install: https://github.com/projectcalico/calico/releases${RESET}"
fi

# ── BGP status ────────────────────────────────────────────────────────────────
if $DO_BGP && [ -n "$CALICO_POD" ]; then
  banner "BGP Peer Status (BIRD)"
  kubectl exec -n kube-system "$CALICO_POD" -- \
    birdcl show protocols all 2>/dev/null | \
    while read -r line; do
      if echo "$line" | grep -qi "established"; then
        echo -e "${GREEN}  ✓ $line${RESET}"
      elif echo "$line" | grep -qi "active\|connect\|idle"; then
        echo -e "${RED}  ✗ $line${RESET}"
      else
        echo "  $line"
      fi
    done
fi

# ── IPAM ─────────────────────────────────────────────────────────────────────
if $DO_IPAM; then
  banner "IPAM Allocation"
  if command -v calicoctl &>/dev/null; then
    calicoctl ipam show --show-blocks 2>/dev/null
  elif [ -n "$CALICO_POD" ]; then
    kubectl exec -n kube-system "$CALICO_POD" -- \
      calicoctl ipam show --show-blocks 2>/dev/null
  fi
fi

# ── Felix logs ────────────────────────────────────────────────────────────────
if [ -n "$CALICO_POD" ]; then
  banner "Recent Felix Errors"
  kubectl logs -n kube-system "$CALICO_POD" -c calico-node --tail=30 2>/dev/null | \
    grep -iE "error|warn|failed|panic" | head -15 | \
    while read -r line; do echo -e "${YELLOW}  $line${RESET}"; done || \
    echo -e "${GREEN}  No errors in recent Felix logs${RESET}"
fi

# ── Policies ──────────────────────────────────────────────────────────────────
banner "Network Policies (Calico)"
if command -v calicoctl &>/dev/null; then
  NP=$(calicoctl get networkpolicy -A 2>/dev/null | wc -l)
  GNP=$(calicoctl get globalnetworkpolicy 2>/dev/null | wc -l)
  echo -e "  Namespace NetworkPolicies: $NP"
  echo -e "  GlobalNetworkPolicies: $GNP"
fi

echo -e "\n${GREEN}Key commands:${RESET}"
echo "  calicoctl node status         → Felix + BGP status"
echo "  calicoctl ipam check          → find leaked IPs"
echo "  calicoctl get globalnetworkpolicy → cluster-wide deny rules"
