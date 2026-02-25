#!/usr/bin/env bash
# k8s-ingress-debug — Ingress/LB/cert-manager diagnostics
# Usage: diagnose.sh [-n ns] [--ingress name] [--certs] [--lb]

set -euo pipefail

NAMESPACE=""
INGRESS=""
CHECK_CERTS=false
CHECK_LB=false

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    -n)         NAMESPACE="$2"; shift 2 ;;
    --ingress)  INGRESS="$2"; shift 2 ;;
    --certs)    CHECK_CERTS=true; shift ;;
    --lb)       CHECK_LB=true; shift ;;
    -h|--help)  echo "Usage: $0 [-n ns] [--ingress name] [--certs] [--lb]"; exit 0 ;;
    *)          shift ;;
  esac
done

NS_FLAG="${NAMESPACE:+-n $NAMESPACE}"
if [ -z "$NAMESPACE" ]; then NS_FLAG="-A"; fi

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║   K8s Ingress Debug — Traffic Routing    ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"

# ── Ingress controllers ───────────────────────────────────────────────────────
banner "Ingress Controllers"
for ns in ingress-nginx traefik ingress-traefik haproxy-controller kong; do
  PODS=$(kubectl get pods -n "$ns" --no-headers 2>/dev/null | wc -l)
  if [ "$PODS" -gt 0 ]; then
    echo -e "${GREEN}  ✓ Found ingress controller in namespace: $ns${RESET}"
    kubectl get pods -n "$ns" --no-headers 2>/dev/null | head -3
  fi
done

# ── IngressClasses ────────────────────────────────────────────────────────────
banner "IngressClasses"
kubectl get ingressclass 2>/dev/null || echo -e "${YELLOW}  No IngressClasses found${RESET}"

# ── All Ingress resources ─────────────────────────────────────────────────────
banner "Ingress Resources"
kubectl get ingress $NS_FLAG 2>/dev/null | while read -r line; do
  if echo "$line" | grep -q "<none>"; then
    echo -e "${YELLOW}  ⚠ (no address) $line${RESET}"
  else
    echo "  $line"
  fi
done

# ── Specific ingress deep-dive ────────────────────────────────────────────────
if [ -n "$INGRESS" ]; then
  NS="${NAMESPACE:-default}"
  banner "Ingress Deep Dive: $INGRESS"
  kubectl describe ingress "$INGRESS" -n "$NS" 2>/dev/null

  # Backend services check
  echo -e "\n${CYAN}Backend Service Endpoints:${RESET}"
  BACKEND_SVC=$(kubectl get ingress "$INGRESS" -n "$NS" \
    -o jsonpath='{.spec.rules[*].http.paths[*].backend.service.name}' 2>/dev/null)
  for svc in $BACKEND_SVC; do
    echo -e "  Service: $svc"
    kubectl get endpoints "$svc" -n "$NS" 2>/dev/null | tail -1 | \
      while read -r line; do
        if echo "$line" | grep -q "<none>"; then
          echo -e "${RED}    ✗ No endpoints — selector mismatch!${RESET}"
        else
          echo -e "${GREEN}    ✓ $line${RESET}"
        fi
      done
  done
fi

# ── LoadBalancer services ─────────────────────────────────────────────────────
if $CHECK_LB || true; then
  banner "LoadBalancer Services"
  kubectl get svc -A 2>/dev/null | grep "LoadBalancer" | while read -r line; do
    if echo "$line" | grep -q "<pending>"; then
      echo -e "${RED}  ✗ (pending IP) $line${RESET}"
    else
      echo -e "${GREEN}  ✓ $line${RESET}"
    fi
  done
fi

# ── cert-manager ─────────────────────────────────────────────────────────────
if $CHECK_CERTS; then
  banner "cert-manager Certificates"
  if kubectl api-resources 2>/dev/null | grep -q "cert-manager.io"; then
    kubectl get certificates $NS_FLAG 2>/dev/null | while read -r line; do
      if echo "$line" | grep -q "False\|False"; then
        echo -e "${RED}  ✗ $line${RESET}"
      elif echo "$line" | grep -q "True"; then
        echo -e "${GREEN}  ✓ $line${RESET}"
      else
        echo "  $line"
      fi
    done
    echo ""
    echo -e "${CYAN}Challenges (ACME):${RESET}"
    kubectl get challenges $NS_FLAG 2>/dev/null | tail -10
  else
    echo -e "${YELLOW}  cert-manager not installed${RESET}"
    echo "  Install: https://cert-manager.io/docs/installation/"
  fi
fi

echo -e "\n${GREEN}Tips:${RESET}"
echo "  Check nginx logs: kubectl logs -n ingress-nginx <pod> --tail=30"
echo "  Test routing:     curl -H 'Host: myapp.example.com' http://<ingress-ip>/"
echo "  Cert status:      kubectl describe certificate <name> -n <ns>"
