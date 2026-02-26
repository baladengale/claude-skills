#!/usr/bin/env bash
# k8s-pod-debug — Pod failure diagnostics
# Usage: diagnose.sh [-n namespace] [-p pod] [-a]
# Inspired by: robusta, komodor, learnk8s troubleshooting guides

set -euo pipefail

NAMESPACE=""
POD=""
ALL_NS=false

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

usage() {
  echo "Usage: $0 [-n namespace] [-p pod-name] [-a]"
  echo "  -n  Namespace to check (default: current context namespace)"
  echo "  -p  Specific pod to deep-dive"
  echo "  -a  Check all namespaces"
  exit 0
}

while getopts "n:p:ah" opt; do
  case $opt in
    n) NAMESPACE="$OPTARG" ;;
    p) POD="$OPTARG" ;;
    a) ALL_NS=true ;;
    h) usage ;;
    *) usage ;;
  esac
done

NS_FLAG=""
if $ALL_NS; then
  NS_FLAG="-A"
elif [ -n "$NAMESPACE" ]; then
  NS_FLAG="-n $NAMESPACE"
fi

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║     K8s Pod Debug — Diagnostics      ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════╝${RESET}"
echo ""

# ── Section 1: Non-running pods ──────────────────────────────────────────────
echo -e "${BOLD}── Unhealthy Pods ──────────────────────────────────────────────────────${RESET}"
kubectl get pods $NS_FLAG \
  --field-selector='status.phase!=Running,status.phase!=Succeeded' \
  --no-headers 2>/dev/null | \
  while read -r line; do
    echo -e "${RED}  ✗ $line${RESET}"
  done
echo ""

# ── Section 2: High restart pods ─────────────────────────────────────────────
echo -e "${BOLD}── High Restart Count (>5) ─────────────────────────────────────────────${RESET}"
kubectl get pods $NS_FLAG --no-headers 2>/dev/null | \
  awk '$NF~/[0-9]/ && $(NF-2)+0 > 5 {print "  ⚠ " $0}' | \
  while read -r line; do echo -e "${YELLOW}$line${RESET}"; done
echo ""

# ── Section 3: Pod-specific deep dive ────────────────────────────────────────
if [ -n "$POD" ]; then
  NS="${NAMESPACE:-$(kubectl config view --minify -o jsonpath='{..namespace}' 2>/dev/null || echo default)}"
  echo -e "${BOLD}── Deep Dive: $POD (ns: $NS) ─────────────────────────────────────────${RESET}"

  echo -e "\n${CYAN}[Status]${RESET}"
  kubectl get pod "$POD" -n "$NS" -o wide 2>/dev/null || echo "  Pod not found"

  echo -e "\n${CYAN}[Container States]${RESET}"
  kubectl get pod "$POD" -n "$NS" \
    -o jsonpath='{range .status.containerStatuses[*]}Container: {.name}  Ready: {.ready}  Restarts: {.restartCount}  State: {.state}{"\n"}{end}' 2>/dev/null

  echo -e "\n${CYAN}[Events]${RESET}"
  kubectl get events -n "$NS" --field-selector="involvedObject.name=$POD" \
    --sort-by='.lastTimestamp' 2>/dev/null | tail -15

  echo -e "\n${CYAN}[Recent Logs]${RESET}"
  kubectl logs "$POD" -n "$NS" --tail=30 2>/dev/null || \
  kubectl logs "$POD" -n "$NS" --tail=30 --previous 2>/dev/null || \
    echo "  No logs available"

  echo -e "\n${CYAN}[Resource Requests/Limits]${RESET}"
  kubectl get pod "$POD" -n "$NS" \
    -o jsonpath='{range .spec.containers[*]}Container: {.name}  Requests: {.resources.requests}  Limits: {.resources.limits}{"\n"}{end}' 2>/dev/null
fi

# ── Section 4: Recent warning events ─────────────────────────────────────────
echo -e "${BOLD}── Recent Warning Events ───────────────────────────────────────────────${RESET}"
kubectl get events $NS_FLAG --field-selector=type=Warning \
  --sort-by='.lastTimestamp' 2>/dev/null | tail -20 | \
  while read -r line; do echo -e "${YELLOW}  $line${RESET}"; done
echo ""

echo -e "${GREEN}Done. Run with -p <pod-name> for deep-dive analysis.${RESET}"
