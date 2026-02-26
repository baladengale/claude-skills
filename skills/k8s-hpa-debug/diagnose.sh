#!/usr/bin/env bash
# k8s-hpa-debug — HPA/VPA/KEDA autoscaling diagnostics
# Usage: diagnose.sh [-n ns] [--hpa name] [--metrics-server] [--keda]

set -euo pipefail

NAMESPACE=""
HPA=""
CHECK_METRICS=false
CHECK_KEDA=false

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    -n)              NAMESPACE="$2"; shift 2 ;;
    --hpa)           HPA="$2"; shift 2 ;;
    --metrics-server)CHECK_METRICS=true; shift ;;
    --keda)          CHECK_KEDA=true; shift ;;
    -h|--help)       echo "Usage: $0 [-n ns] [--hpa name] [--metrics-server] [--keda]"; exit 0 ;;
    *)               shift ;;
  esac
done

NS_FLAG="${NAMESPACE:+-n $NAMESPACE}"
if [ -z "$NAMESPACE" ]; then NS_FLAG="-A"; fi

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║   K8s HPA Debug — Autoscaling Status     ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"

# ── Metrics server check ──────────────────────────────────────────────────────
banner "Metrics Server"
if kubectl top nodes &>/dev/null; then
  echo -e "${GREEN}  ✓ metrics-server is working${RESET}"
  kubectl top nodes 2>/dev/null | head -5
else
  echo -e "${RED}  ✗ metrics-server not working — HPA cannot get CPU/memory metrics${RESET}"
  echo -e "${YELLOW}  Install: kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml${RESET}"
fi

# ── HPA Status ────────────────────────────────────────────────────────────────
banner "HPA Status"
kubectl get hpa $NS_FLAG 2>/dev/null | while read -r line; do
  if echo "$line" | grep -q "<unknown>"; then
    echo -e "${RED}  ✗ (metrics missing) $line${RESET}"
  elif echo "$line" | grep -qE "MAXPODS|NAME"; then
    echo "  $line"
  else
    echo -e "${GREEN}  ✓ $line${RESET}"
  fi
done

# ── HPA deep-dive ─────────────────────────────────────────────────────────────
if [ -n "$HPA" ]; then
  NS="${NAMESPACE:-default}"
  banner "HPA Deep Dive: $HPA"
  kubectl describe hpa "$HPA" -n "$NS" 2>/dev/null
fi

# ── VPA status ────────────────────────────────────────────────────────────────
banner "VPA (Vertical Pod Autoscaler)"
if kubectl api-resources 2>/dev/null | grep -q "verticalpodautoscalers"; then
  kubectl get vpa $NS_FLAG 2>/dev/null || echo -e "${YELLOW}  No VPA objects found${RESET}"
else
  echo -e "${YELLOW}  VPA not installed (optional)${RESET}"
  echo "  Install: https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler"
fi

# ── KEDA check ────────────────────────────────────────────────────────────────
if $CHECK_KEDA; then
  banner "KEDA ScaledObjects"
  if kubectl api-resources 2>/dev/null | grep -q "scaledobject"; then
    kubectl get scaledobject $NS_FLAG 2>/dev/null
    echo ""
    kubectl get scaledjob $NS_FLAG 2>/dev/null
  else
    echo -e "${YELLOW}  KEDA not installed${RESET}"
    echo "  Install: https://keda.sh/docs/latest/deploy/"
  fi
fi

# ── Pods without resource requests ────────────────────────────────────────────
if [ -n "$NAMESPACE" ]; then
  banner "Pods Without CPU Requests (breaks CPU-based HPA)"
  kubectl get pods -n "$NAMESPACE" \
    -o json 2>/dev/null | \
    jq -r '.items[] |
      select(.spec.containers[].resources.requests.cpu == null) |
      "  ⚠ " + .metadata.name + " — no CPU request set"' | \
    while read -r line; do echo -e "${YELLOW}$line${RESET}"; done || true
fi

echo -e "\n${GREEN}Key commands:${RESET}"
echo "  kubectl describe hpa <name> -n <ns>   → see scaling events and conditions"
echo "  kubectl top pods -n <ns>              → current CPU/memory usage"
echo "  kubectl get hpa -n <ns> -w            → watch scaling in real time"
