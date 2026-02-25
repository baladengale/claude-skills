#!/usr/bin/env bash
# k8s-network-debug — Kubernetes networking diagnostics
# Usage: diagnose.sh -n <namespace> [--dns] [--service <svc>] [--policy] [--all]
# Tools: netshoot (nicolaka/netshoot), kubectl

set -euo pipefail

NAMESPACE="default"
CHECK_DNS=false
CHECK_SVC=""
CHECK_POLICY=false
ALL_CHECKS=false
POD=""

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

usage() {
  echo "Usage: $0 -n <namespace> [options]"
  echo "  -n <ns>         Namespace to inspect"
  echo "  --dns           Test DNS resolution"
  echo "  --service <s>   Test service endpoints"
  echo "  --pod <p>       Pod to inspect"
  echo "  --policy        Audit NetworkPolicies"
  echo "  --all           Run all checks"
  exit 0
}

while [[ $# -gt 0 ]]; do
  case $1 in
    -n)          NAMESPACE="$2"; shift 2 ;;
    --dns)       CHECK_DNS=true; shift ;;
    --service)   CHECK_SVC="$2"; shift 2 ;;
    --pod)       POD="$2"; shift 2 ;;
    --policy)    CHECK_POLICY=true; shift ;;
    --all)       ALL_CHECKS=true; shift ;;
    -h|--help)   usage ;;
    *)           echo "Unknown flag: $1"; usage ;;
  esac
done

banner() { echo -e "${BOLD}${CYAN}── $1 ──────────────────────────────────────────────────────────────${RESET}"; }

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║   K8s Network Debug — Namespace: $NAMESPACE   ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"
echo ""

# ── CoreDNS health ───────────────────────────────────────────────────────────
banner "CoreDNS Health"
kubectl get pods -n kube-system -l k8s-app=kube-dns -o wide 2>/dev/null
echo ""

# ── Services and Endpoints ───────────────────────────────────────────────────
banner "Services & Endpoints (ns: $NAMESPACE)"
echo -e "${YELLOW}Services:${RESET}"
kubectl get svc -n "$NAMESPACE" -o wide 2>/dev/null
echo ""
echo -e "${YELLOW}Endpoints with no ready addresses:${RESET}"
kubectl get endpoints -n "$NAMESPACE" 2>/dev/null | awk 'NR==1 || $2=="<none>"' | \
  while read -r line; do echo -e "${RED}  ✗ $line${RESET}"; done
echo ""

# ── Specific service check ────────────────────────────────────────────────────
if [ -n "$CHECK_SVC" ] || $ALL_CHECKS; then
  SVC_NAME="${CHECK_SVC:-}"
  if [ -n "$SVC_NAME" ]; then
    banner "Service Detail: $SVC_NAME"
    echo -e "${CYAN}Selector:${RESET}"
    kubectl get svc "$SVC_NAME" -n "$NAMESPACE" -o jsonpath='{.spec.selector}' 2>/dev/null && echo ""
    echo -e "\n${CYAN}Endpoints:${RESET}"
    kubectl get endpoints "$SVC_NAME" -n "$NAMESPACE" 2>/dev/null
    echo -e "\n${CYAN}Pods matching selector:${RESET}"
    SEL=$(kubectl get svc "$SVC_NAME" -n "$NAMESPACE" \
      -o jsonpath='{range .spec.selector}{@k}={@v},{end}' 2>/dev/null | sed 's/,$//')
    if [ -n "$SEL" ]; then
      kubectl get pods -n "$NAMESPACE" -l "$SEL" -o wide 2>/dev/null || \
        echo -e "${RED}  No pods match selector '$SEL' — selector mismatch!${RESET}"
    fi
    echo ""
  fi
fi

# ── NetworkPolicy audit ───────────────────────────────────────────────────────
if $CHECK_POLICY || $ALL_CHECKS; then
  banner "NetworkPolicy Audit (ns: $NAMESPACE)"
  NP_COUNT=$(kubectl get networkpolicy -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l)
  if [ "$NP_COUNT" -eq 0 ]; then
    echo -e "${GREEN}  No NetworkPolicies — all traffic allowed${RESET}"
  else
    echo -e "${YELLOW}  $NP_COUNT NetworkPolicy(ies) found:${RESET}"
    kubectl get networkpolicy -n "$NAMESPACE" 2>/dev/null
    echo ""
    kubectl describe networkpolicy -n "$NAMESPACE" 2>/dev/null | \
      grep -E "Name:|PodSelector:|Ingress|Egress|Port|From|To" | head -40
  fi
  echo ""
fi

# ── DNS test ──────────────────────────────────────────────────────────────────
if $CHECK_DNS || $ALL_CHECKS; then
  banner "DNS Resolution Test"
  echo -e "${CYAN}Launching netshoot pod for DNS testing...${RESET}"
  echo -e "  nslookup kubernetes.default.svc.cluster.local"
  kubectl run dns-test-$$ --rm -i --restart=Never \
    --image=nicolaka/netshoot \
    -n "$NAMESPACE" \
    --timeout=60s \
    -- nslookup kubernetes.default.svc.cluster.local 2>/dev/null || \
    echo -e "${YELLOW}  netshoot image pull may take a moment. Re-run or use: kubectl run netshoot --rm -it -n $NAMESPACE --image=nicolaka/netshoot -- bash${RESET}"
  echo ""
fi

# ── Network summary advice ────────────────────────────────────────────────────
banner "Diagnostic Summary"
echo -e "${CYAN}Common next steps:${RESET}"
echo "  1. If service has no endpoints → check pod selector and pod readiness"
echo "  2. If DNS fails → check CoreDNS pods and configmap"
echo "  3. If blocked by NetworkPolicy → review ingress/egress rules"
echo "  4. For live packet inspection:"
echo "     kubectl run netshoot --rm -it -n $NAMESPACE --image=nicolaka/netshoot -- tcpdump -i eth0"
echo ""
echo -e "${GREEN}Run with --all for comprehensive checks.${RESET}"
