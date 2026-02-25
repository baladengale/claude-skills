#!/usr/bin/env bash
# envoy-gateway-debug — Envoy Gateway and Gateway API diagnostics
# Usage: diagnose.sh [-n ns] [--control-plane] [--admin --pod <name>]

set -euo pipefail

NAMESPACE=""
CHECK_CP=false
ADMIN_POD=""

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    -n)              NAMESPACE="$2"; shift 2 ;;
    --control-plane) CHECK_CP=true; shift ;;
    --pod)           ADMIN_POD="$2"; shift 2 ;;
    --admin)         shift ;;  # flag is implied by --pod
    -h|--help)       echo "Usage: $0 [-n ns] [--control-plane] [--pod <envoy-pod> -n <ns>]"; exit 0 ;;
    *)               shift ;;
  esac
done

NS_FLAG="${NAMESPACE:+-n $NAMESPACE}"
if [ -z "$NAMESPACE" ]; then NS_FLAG="-A"; fi

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║  Envoy Gateway Debug — Gateway API       ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"

# ── GatewayClass ──────────────────────────────────────────────────────────────
banner "GatewayClasses"
kubectl get gatewayclass 2>/dev/null || echo -e "${YELLOW}  Gateway API CRDs not installed${RESET}"

# ── Gateways ──────────────────────────────────────────────────────────────────
banner "Gateways"
kubectl get gateway $NS_FLAG 2>/dev/null | while read -r line; do
  if echo "$line" | grep -qE "False|Unknown"; then
    echo -e "${YELLOW}  ⚠ $line${RESET}"
  else
    echo "  $line"
  fi
done

# ── HTTPRoutes ────────────────────────────────────────────────────────────────
banner "HTTPRoutes"
kubectl get httproute $NS_FLAG 2>/dev/null | while read -r line; do
  if echo "$line" | grep -qE "False|Unknown"; then
    echo -e "${RED}  ✗ $line${RESET}"
  else
    echo "  $line"
  fi
done

# ── Envoy Gateway control plane ───────────────────────────────────────────────
if $CHECK_CP; then
  banner "Envoy Gateway Control Plane"
  for ns in envoy-gateway-system gateway-system; do
    PODS=$(kubectl get pods -n "$ns" --no-headers 2>/dev/null)
    if [ -n "$PODS" ]; then
      echo -e "${GREEN}  Found Envoy Gateway in namespace: $ns${RESET}"
      echo "$PODS"
      echo ""
      echo -e "${CYAN}  Recent logs:${RESET}"
      kubectl logs -n "$ns" deployment/envoy-gateway --tail=20 2>/dev/null | \
        grep -iE "error|warn|fail" | head -10
    fi
  done
fi

# ── Admin API guidance ────────────────────────────────────────────────────────
if [ -n "$ADMIN_POD" ] && [ -n "$NAMESPACE" ]; then
  banner "Envoy Admin Access: $ADMIN_POD"
  echo -e "${CYAN}  Port-forwarding to admin port 9901...${RESET}"
  echo -e "  kubectl port-forward -n $NAMESPACE $ADMIN_POD 9901:9901"
  echo -e "\n  Then in another terminal:"
  echo "  curl http://localhost:9901/config_dump | jq ."
  echo "  curl http://localhost:9901/stats | grep upstream"
  echo "  curl http://localhost:9901/clusters"
fi

# ── Envoy proxy pods ──────────────────────────────────────────────────────────
banner "Envoy Proxy Pods"
kubectl get pods $NS_FLAG 2>/dev/null | grep -i "envoy\|gateway-proxy" | \
  while read -r line; do
    if echo "$line" | grep -qE "0/|Error|CrashLoop"; then
      echo -e "${RED}  ✗ $line${RESET}"
    else
      echo "  $line"
    fi
  done

echo -e "\n${GREEN}Tips:${RESET}"
echo "  HTTPRoute not routing → check gateway status: kubectl describe gateway <name> -n <ns>"
echo "  Backend unreachable   → verify endpoints: kubectl get endpoints <svc> -n <ns>"
echo "  Envoy config dump     → kubectl port-forward <envoy-pod> 9901:9901; curl localhost:9901/config_dump"
