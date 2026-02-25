#!/usr/bin/env bash
# etcd-debug — etcd cluster diagnostics
# Usage: diagnose.sh [--health] [--perf] [--backup <path>] [--defrag]
# Must run on a control-plane node with etcd certs available

set -euo pipefail

DO_HEALTH=false
DO_PERF=false
BACKUP_PATH=""
DO_DEFRAG=false

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    --health)   DO_HEALTH=true; shift ;;
    --perf)     DO_PERF=true; shift ;;
    --backup)   BACKUP_PATH="$2"; shift 2 ;;
    --defrag)   DO_DEFRAG=true; shift ;;
    -h|--help)  echo "Usage: $0 [--health] [--perf] [--backup <path>] [--defrag]"; exit 0 ;;
    *)          shift ;;
  esac
done

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║      etcd Debug — Cluster Diagnostics    ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"

# ── Detect etcd access ────────────────────────────────────────────────────────
banner "Detecting etcd Access Method"

# Try to find etcdctl
if ! command -v etcdctl &>/dev/null; then
  echo -e "${YELLOW}  etcdctl not in PATH. Trying via kubectl exec into etcd pod...${RESET}"
  ETCD_POD=$(kubectl get pods -n kube-system -l component=etcd -o name 2>/dev/null | head -1 || \
             kubectl get pods -n openshift-etcd -l app=etcd -o name 2>/dev/null | head -1 || true)
  if [ -z "$ETCD_POD" ]; then
    echo -e "${RED}  Cannot find etcd pod. Run this script directly on a control-plane node.${RESET}"
    exit 1
  fi
  echo -e "${GREEN}  Found etcd pod: $ETCD_POD${RESET}"
  echo -e "${CYAN}  Use: kubectl exec -n kube-system $ETCD_POD -- etcdctl member list${RESET}"
  exit 0
fi

# Auto-detect certs (kubeadm default paths)
ETCDCTL_API="${ETCDCTL_API:-3}"
ETCDCTL_ENDPOINTS="${ETCDCTL_ENDPOINTS:-https://127.0.0.1:2379}"
ETCDCTL_CACERT="${ETCDCTL_CACERT:-/etc/kubernetes/pki/etcd/ca.crt}"
ETCDCTL_CERT="${ETCDCTL_CERT:-/etc/kubernetes/pki/etcd/server.crt}"
ETCDCTL_KEY="${ETCDCTL_KEY:-/etc/kubernetes/pki/etcd/server.key}"

export ETCDCTL_API ETCDCTL_ENDPOINTS ETCDCTL_CACERT ETCDCTL_CERT ETCDCTL_KEY

ETCD_ARGS="--endpoints=$ETCDCTL_ENDPOINTS --cacert=$ETCDCTL_CACERT --cert=$ETCDCTL_CERT --key=$ETCDCTL_KEY"

echo -e "${GREEN}  etcdctl found. Endpoints: $ETCDCTL_ENDPOINTS${RESET}"

# ── Health check ──────────────────────────────────────────────────────────────
banner "etcd Cluster Status"
etcdctl endpoint status --cluster -w table 2>/dev/null || \
  echo -e "${RED}  Cannot reach etcd endpoints. Check certs and network.${RESET}"

banner "Member Health"
etcdctl endpoint health --cluster 2>/dev/null | while read -r line; do
  if echo "$line" | grep -q "unhealthy"; then
    echo -e "${RED}  ✗ $line${RESET}"
  else
    echo -e "${GREEN}  ✓ $line${RESET}"
  fi
done

banner "Alarms"
ALARMS=$(etcdctl alarm list 2>/dev/null)
if [ -z "$ALARMS" ]; then
  echo -e "${GREEN}  No alarms${RESET}"
else
  echo -e "${RED}  $ALARMS${RESET}"
fi

# ── Performance check ─────────────────────────────────────────────────────────
if $DO_PERF; then
  banner "Performance Check"
  echo -e "${CYAN}Writing test (checks disk latency):${RESET}"
  etcdctl check perf 2>/dev/null || echo -e "${YELLOW}  check perf not available on this etcdctl version${RESET}"

  echo -e "\n${CYAN}DB sizes:${RESET}"
  etcdctl endpoint status --cluster -w json 2>/dev/null | \
    jq -r '.[] | .Endpoint + " db_size=" + (.Status.dbSize | tostring) + " bytes"'
fi

# ── Defragmentation ───────────────────────────────────────────────────────────
if $DO_DEFRAG; then
  banner "Defragmentation"
  echo -e "${YELLOW}  Running defrag on all members (one at a time)...${RESET}"
  etcdctl defrag --cluster 2>/dev/null && \
    echo -e "${GREEN}  Defrag complete${RESET}" || \
    echo -e "${RED}  Defrag failed — check logs${RESET}"
fi

# ── Backup ────────────────────────────────────────────────────────────────────
if [ -n "$BACKUP_PATH" ]; then
  banner "Snapshot Backup"
  echo -e "${CYAN}  Saving snapshot to: $BACKUP_PATH${RESET}"
  etcdctl snapshot save "$BACKUP_PATH" 2>/dev/null && \
    etcdctl snapshot status "$BACKUP_PATH" -w table && \
    echo -e "${GREEN}  Snapshot saved successfully${RESET}" || \
    echo -e "${RED}  Snapshot failed${RESET}"
fi

echo -e "\n${GREEN}Tips:${RESET}"
echo "  Slow API server → check etcd disk latency with: fio --rw=write --ioengine=sync --fdatasync=1 --directory=/var/lib/etcd --size=22m --bs=2300 --name=test"
echo "  DB too large   → compact + defrag + alarm disarm"
echo "  Member down    → etcdctl member remove <id> + add replacement before quorum loss"
