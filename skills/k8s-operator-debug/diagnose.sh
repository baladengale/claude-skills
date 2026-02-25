#!/usr/bin/env bash
# k8s-operator-debug — Operator and CRD diagnostics
# Usage: diagnose.sh [--all] [--operator <name> -n <ns>] [--terminating] [--webhooks]

set -euo pipefail

ALL=false
OPERATOR=""
NAMESPACE=""
CHECK_TERM=false
CHECK_WEBHOOKS=false

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    --all)          ALL=true; CHECK_TERM=true; CHECK_WEBHOOKS=true; shift ;;
    --operator)     OPERATOR="$2"; shift 2 ;;
    -n)             NAMESPACE="$2"; shift 2 ;;
    --terminating)  CHECK_TERM=true; shift ;;
    --webhooks)     CHECK_WEBHOOKS=true; shift ;;
    -h|--help)      echo "Usage: $0 [--all] [--operator <name> -n <ns>] [--terminating] [--webhooks]"; exit 0 ;;
    *)              shift ;;
  esac
done

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║  K8s Operator Debug — Operator Health    ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"

# ── CRDs ──────────────────────────────────────────────────────────────────────
banner "Installed CRDs (non-system)"
kubectl get crd 2>/dev/null | grep -v "k8s.io\|kubernetes.io" | head -20

# ── Operator pods ─────────────────────────────────────────────────────────────
banner "Operator/Controller Pods"
kubectl get pods -A --no-headers 2>/dev/null | \
  grep -iE "operator|controller|manager" | \
  while read -r line; do
    if echo "$line" | grep -qE "0/[0-9]|Error|CrashLoop"; then
      echo -e "${RED}  ✗ $line${RESET}"
    elif echo "$line" | grep -q "Running"; then
      echo -e "${GREEN}  ✓ $line${RESET}"
    else
      echo "  $line"
    fi
  done

# ── Specific operator logs ────────────────────────────────────────────────────
if [ -n "$OPERATOR" ] && [ -n "$NAMESPACE" ]; then
  banner "Operator Logs: $OPERATOR (ns: $NAMESPACE)"
  kubectl logs -n "$NAMESPACE" "deployment/$OPERATOR" --tail=40 2>/dev/null | \
    grep -iE "error|fail|panic|warn|ERROR|FAIL" | head -20 | \
    while read -r line; do echo -e "${YELLOW}  $line${RESET}"; done || \
    echo -e "${GREEN}  No errors in recent logs${RESET}"
fi

# ── Objects stuck in Terminating ─────────────────────────────────────────────
if $CHECK_TERM; then
  banner "Objects Stuck in Terminating (Finalizer Deadlock)"
  kubectl get ns --no-headers 2>/dev/null | awk '$2=="Terminating" {print "  ns: "$1}' | \
    while read -r line; do echo -e "${RED}$line${RESET}"; done

  kubectl get all -A --no-headers 2>/dev/null | awk '$3=="Terminating" {print "  "$1" "$2" "$3}' | \
    while read -r line; do echo -e "${RED}$line${RESET}"; done

  echo -e "${CYAN}  Fix with: kubectl patch <resource> <name> -p '{\"metadata\":{\"finalizers\":[]}}' --type=merge${RESET}"
fi

# ── Webhooks ──────────────────────────────────────────────────────────────────
if $CHECK_WEBHOOKS; then
  banner "Admission Webhooks"
  echo -e "${CYAN}ValidatingWebhookConfigurations:${RESET}"
  kubectl get validatingwebhookconfiguration 2>/dev/null | head -10
  echo -e "\n${CYAN}MutatingWebhookConfigurations:${RESET}"
  kubectl get mutatingwebhookconfiguration 2>/dev/null | head -10

  # Check if webhook services are reachable
  echo -e "\n${CYAN}Webhook services:${RESET}"
  kubectl get validatingwebhookconfiguration -o json 2>/dev/null | \
    jq -r '.items[].webhooks[].clientConfig.service |
      select(. != null) |
      "  Namespace: " + .namespace + "  Service: " + .name' | sort -u | head -10
fi

echo -e "\n${GREEN}Tips:${RESET}"
echo "  Finalizer deadlock: kubectl patch <res> <name> -p '{\"metadata\":{\"finalizers\":[]}}' --type=merge"
echo "  Webhook blocking:   kubectl patch validatingwebhookconfiguration <name> --type=json -p='[{\"op\":\"replace\",\"path\":\"/webhooks/0/failurePolicy\",\"value\":\"Ignore\"}]'"
echo "  Debug operator:     kubectl logs -n <ns> deployment/<operator> -f | grep -iE 'error|reconcile'"
