#!/usr/bin/env bash
# istio-debug — Istio service mesh diagnostics
# Usage: diagnose.sh [--control-plane] [-n ns] [--proxy-status] [--pod <pod>] [--mtls] [--analyze]
# Requires: istioctl, kubectl

set -euo pipefail

NAMESPACE=""
POD=""
CHECK_CP=false
CHECK_PROXY=false
CHECK_MTLS=false
CHECK_ENVOY=false
CHECK_ANALYZE=false

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }
ok()     { echo -e "${GREEN}  ✓ $1${RESET}"; }
warn()   { echo -e "${YELLOW}  ⚠ $1${RESET}"; }
fail()   { echo -e "${RED}  ✗ $1${RESET}"; }

usage() {
  echo "Usage: $0 [options]"
  echo "  --control-plane   Check Istio control plane"
  echo "  -n <ns>           Namespace"
  echo "  --proxy-status    Check xDS sync status"
  echo "  --pod <pod>       Target pod for Envoy config"
  echo "  --envoy-config    Dump Envoy config for pod"
  echo "  --mtls            Check mTLS policies"
  echo "  --analyze         Run istioctl analyze"
  exit 0
}

while [[ $# -gt 0 ]]; do
  case $1 in
    --control-plane) CHECK_CP=true; shift ;;
    -n)              NAMESPACE="$2"; shift 2 ;;
    --proxy-status)  CHECK_PROXY=true; shift ;;
    --pod)           POD="$2"; shift 2 ;;
    --envoy-config)  CHECK_ENVOY=true; shift ;;
    --mtls)          CHECK_MTLS=true; shift ;;
    --analyze)       CHECK_ANALYZE=true; shift ;;
    -h|--help)       usage ;;
    *)               echo "Unknown: $1"; usage ;;
  esac
done

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║     Istio Debug — Mesh Diagnostics       ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"

# ── Check istioctl available ──────────────────────────────────────────────────
if ! command -v istioctl &>/dev/null; then
  warn "istioctl not found. Install: https://istio.io/latest/docs/setup/getting-started/"
  warn "Falling back to kubectl only..."
fi

# ── Control plane ─────────────────────────────────────────────────────────────
if $CHECK_CP; then
  banner "Control Plane Health (istio-system)"
  kubectl get pods -n istio-system -o wide 2>/dev/null
  echo ""
  echo -e "${CYAN}Istiod logs (last 20 lines):${RESET}"
  kubectl logs -n istio-system -l app=istiod --tail=20 2>/dev/null | grep -E "error|warn|ERR|WARN" || \
    ok "No errors in recent Istiod logs"
fi

# ── Proxy sync status ─────────────────────────────────────────────────────────
if $CHECK_PROXY; then
  banner "Proxy Sync Status (xDS)"
  if command -v istioctl &>/dev/null; then
    istioctl proxy-status 2>/dev/null | while read -r line; do
      if echo "$line" | grep -q "STALE\|NOT SENT"; then
        warn "$line"
      else
        echo "  $line"
      fi
    done
  else
    warn "istioctl required for proxy-status"
  fi
fi

# ── Envoy config dump ─────────────────────────────────────────────────────────
if [ -n "$POD" ] && $CHECK_ENVOY; then
  NS="${NAMESPACE:-default}"
  banner "Envoy Config: $POD (ns: $NS)"
  if command -v istioctl &>/dev/null; then
    echo -e "${CYAN}Clusters:${RESET}"
    istioctl proxy-config clusters "$POD" -n "$NS" 2>/dev/null | head -20
    echo -e "\n${CYAN}Listeners:${RESET}"
    istioctl proxy-config listeners "$POD" -n "$NS" 2>/dev/null | head -20
    echo -e "\n${CYAN}Routes:${RESET}"
    istioctl proxy-config routes "$POD" -n "$NS" 2>/dev/null | head -20
    echo -e "\n${CYAN}Endpoints:${RESET}"
    istioctl proxy-config endpoints "$POD" -n "$NS" 2>/dev/null | head -20
  else
    warn "istioctl required for Envoy config inspection"
  fi
fi

# ── mTLS check ────────────────────────────────────────────────────────────────
if $CHECK_MTLS; then
  NS="${NAMESPACE:-}"
  NS_FLAG="${NAMESPACE:+-n $NAMESPACE}"
  banner "mTLS Policies"
  echo -e "${CYAN}PeerAuthentication policies:${RESET}"
  kubectl get peerauthentication -A 2>/dev/null || warn "No PeerAuthentication found"
  echo ""
  echo -e "${CYAN}DestinationRules (TLS settings):${RESET}"
  kubectl get destinationrule $NS_FLAG 2>/dev/null | head -20
fi

# ── Analyze ───────────────────────────────────────────────────────────────────
if $CHECK_ANALYZE; then
  banner "Istio Analyze"
  if command -v istioctl &>/dev/null; then
    NS_FLAG="${NAMESPACE:+-n $NAMESPACE}"
    if [ -z "$NAMESPACE" ]; then
      NS_FLAG="--all-namespaces"
    fi
    istioctl analyze $NS_FLAG 2>/dev/null
  else
    warn "istioctl required for analyze"
  fi
fi

# ── VirtualServices + DestinationRules ───────────────────────────────────────
NS_FLAG="${NAMESPACE:+-n $NAMESPACE}"
if [ -z "$NAMESPACE" ]; then NS_FLAG="-A"; fi

banner "Traffic Config Summary"
echo -e "${CYAN}VirtualServices:${RESET}"
kubectl get virtualservice $NS_FLAG 2>/dev/null
echo ""
echo -e "${CYAN}DestinationRules:${RESET}"
kubectl get destinationrule $NS_FLAG 2>/dev/null
echo ""
echo -e "${CYAN}Gateways:${RESET}"
kubectl get gateway $NS_FLAG 2>/dev/null

echo -e "\n${GREEN}Done. Run 'istioctl analyze --all-namespaces' for config validation.${RESET}"
