#!/usr/bin/env bash
# kube-proxy-debug — Service routing and iptables diagnostics
# Usage: diagnose.sh [--service <name> -n <ns>] [--conntrack]
# Note: iptables/ipvs checks require node access

set -euo pipefail

SVC=""
NAMESPACE=""
CHECK_CONNTRACK=false

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    --service)    SVC="$2"; shift 2 ;;
    -n)           NAMESPACE="$2"; shift 2 ;;
    --conntrack)  CHECK_CONNTRACK=true; shift ;;
    -h|--help)    echo "Usage: $0 [--service <name> -n <ns>] [--conntrack]"; exit 0 ;;
    *)            shift ;;
  esac
done

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║  kube-proxy Debug — Service Routing      ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"

# ── kube-proxy pods ───────────────────────────────────────────────────────────
banner "kube-proxy DaemonSet Health"
kubectl get pods -n kube-system -l k8s-app=kube-proxy -o wide 2>/dev/null | \
  while read -r line; do
    if echo "$line" | grep -qE "0/|Error|CrashLoop"; then
      echo -e "${RED}  ✗ $line${RESET}"
    elif echo "$line" | grep -q "Running"; then
      echo -e "${GREEN}  ✓ $line${RESET}"
    else
      echo "  $line"
    fi
  done

# ── kube-proxy mode ───────────────────────────────────────────────────────────
banner "kube-proxy Mode"
MODE=$(kubectl get configmap kube-proxy -n kube-system -o jsonpath='{.data.config\.conf}' 2>/dev/null | \
  grep "^mode:" | awk '{print $2}')
echo -e "  Mode: ${CYAN}${MODE:-iptables (default)}${RESET}"

# ── Recent errors ─────────────────────────────────────────────────────────────
banner "kube-proxy Recent Errors"
KP_POD=$(kubectl get pod -n kube-system -l k8s-app=kube-proxy -o name 2>/dev/null | head -1)
if [ -n "$KP_POD" ]; then
  kubectl logs "$KP_POD" -n kube-system --tail=30 2>/dev/null | \
    grep -iE "error|fail|warn|panic" | head -10 | \
    while read -r line; do echo -e "${YELLOW}  $line${RESET}"; done || \
    echo -e "${GREEN}  No errors in recent logs${RESET}"
fi

# ── Service verification ──────────────────────────────────────────────────────
if [ -n "$SVC" ] && [ -n "$NAMESPACE" ]; then
  banner "Service: $SVC (ns: $NAMESPACE)"
  kubectl get svc "$SVC" -n "$NAMESPACE" 2>/dev/null
  echo ""
  echo -e "${CYAN}Endpoints:${RESET}"
  kubectl get endpoints "$SVC" -n "$NAMESPACE" 2>/dev/null

  CLUSTER_IP=$(kubectl get svc "$SVC" -n "$NAMESPACE" \
    -o jsonpath='{.spec.clusterIP}' 2>/dev/null)
  echo -e "\n${CYAN}ClusterIP: $CLUSTER_IP${RESET}"
  echo -e "${YELLOW}  To verify iptables rules on a node:${RESET}"
  echo "  kubectl debug node/<node> -it --image=ubuntu"
  echo "  chroot /host && iptables -L KUBE-SERVICES -n | grep $CLUSTER_IP"
fi

# ── Conntrack info ────────────────────────────────────────────────────────────
if $CHECK_CONNTRACK; then
  banner "conntrack Table (via node)"
  echo -e "${CYAN}To check conntrack on a node:${RESET}"
  echo "  kubectl debug node/<node> -it --image=ubuntu"
  echo "  chroot /host"
  echo "  cat /proc/sys/net/netfilter/nf_conntrack_count  # current entries"
  echo "  cat /proc/sys/net/netfilter/nf_conntrack_max    # maximum entries"
  echo "  # If count ≈ max → INCREASE: sysctl -w net.netfilter.nf_conntrack_max=524288"
fi

# ── eBPF replacement check ────────────────────────────────────────────────────
banner "kube-proxy Replacement (eBPF CNIs)"
CILIUM_POD=$(kubectl get pod -n kube-system -l k8s-app=cilium -o name 2>/dev/null | head -1)
if [ -n "$CILIUM_POD" ]; then
  echo -e "${CYAN}Cilium detected — checking kube-proxy replacement:${RESET}"
  kubectl exec "$CILIUM_POD" -n kube-system -- \
    cilium status 2>/dev/null | grep -i "kubeproxy\|kube-proxy" || \
    echo "  Run: cilium status inside the cilium pod"
fi

echo -e "\n${GREEN}Key checks:${RESET}"
echo "  iptables rules exist for service: kubectl debug node/<n> -it --image=ubuntu"
echo "  conntrack overflow: /proc/sys/net/netfilter/nf_conntrack_count vs _max"
echo "  kube-proxy logs:    kubectl logs -n kube-system <kube-proxy-pod>"
