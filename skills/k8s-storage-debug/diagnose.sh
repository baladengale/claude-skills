#!/usr/bin/env bash
# k8s-storage-debug — PV/PVC/StorageClass diagnostics
# Usage: diagnose.sh [-n <namespace>] [--pvc <name>]

set -euo pipefail

NAMESPACE=""
PVC=""

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    -n)      NAMESPACE="$2"; shift 2 ;;
    --pvc)   PVC="$2"; shift 2 ;;
    -h)      echo "Usage: $0 [-n ns] [--pvc name]"; exit 0 ;;
    *)       shift ;;
  esac
done

NS_FLAG="${NAMESPACE:+-n $NAMESPACE}"
if [ -z "$NAMESPACE" ]; then NS_FLAG="-A"; fi

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║   K8s Storage Debug — PV/PVC Status      ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"

# ── StorageClasses ────────────────────────────────────────────────────────────
banner "StorageClasses"
kubectl get storageclass 2>/dev/null | while read -r line; do
  if echo "$line" | grep -q "(default)"; then
    echo -e "${GREEN}  ✓ $line${RESET}"
  else
    echo "  $line"
  fi
done

# ── PV Status ─────────────────────────────────────────────────────────────────
banner "PersistentVolumes"
kubectl get pv 2>/dev/null | while read -r line; do
  if echo "$line" | grep -qE "Released|Failed|Available"; then
    echo -e "${YELLOW}  ⚠ $line${RESET}"
  elif echo "$line" | grep -q "Bound"; then
    echo -e "${GREEN}  ✓ $line${RESET}"
  else
    echo "  $line"
  fi
done

# ── PVC Status ────────────────────────────────────────────────────────────────
banner "PersistentVolumeClaims"
kubectl get pvc $NS_FLAG 2>/dev/null | while read -r line; do
  if echo "$line" | grep -q "Pending"; then
    echo -e "${RED}  ✗ $line${RESET}"
  elif echo "$line" | grep -q "Lost"; then
    echo -e "${RED}  ✗ $line${RESET}"
  elif echo "$line" | grep -q "Bound"; then
    echo -e "${GREEN}  ✓ $line${RESET}"
  else
    echo "  $line"
  fi
done

# ── Pending PVC detail ────────────────────────────────────────────────────────
PENDING_PVCS=$(kubectl get pvc $NS_FLAG --no-headers 2>/dev/null | grep "Pending" | awk '{print $1}' || true)
if [ -n "$PENDING_PVCS" ]; then
  banner "Pending PVC Events"
  NS_OPT="${NAMESPACE:-default}"
  for pvc_name in $PENDING_PVCS; do
    echo -e "${RED}  PVC: $pvc_name${RESET}"
    kubectl get events -n "$NS_OPT" \
      --field-selector="involvedObject.name=$pvc_name" \
      --sort-by='.lastTimestamp' 2>/dev/null | tail -5
    echo ""
  done
fi

# ── Specific PVC deep-dive ────────────────────────────────────────────────────
if [ -n "$PVC" ]; then
  NS="${NAMESPACE:-default}"
  banner "PVC Deep Dive: $PVC (ns: $NS)"
  kubectl describe pvc "$PVC" -n "$NS" 2>/dev/null

  # Find the bound PV
  PV_NAME=$(kubectl get pvc "$PVC" -n "$NS" -o jsonpath='{.spec.volumeName}' 2>/dev/null)
  if [ -n "$PV_NAME" ]; then
    echo -e "\n${CYAN}Bound PV: $PV_NAME${RESET}"
    kubectl describe pv "$PV_NAME" 2>/dev/null | head -30
  fi
fi

# ── VolumeAttachments ─────────────────────────────────────────────────────────
banner "VolumeAttachments"
kubectl get volumeattachments 2>/dev/null | while read -r line; do
  if echo "$line" | grep -q "false"; then
    echo -e "${YELLOW}  ⚠ (not attached) $line${RESET}"
  else
    echo "  $line"
  fi
done

# ── CSI Drivers ───────────────────────────────────────────────────────────────
banner "CSI Drivers"
kubectl get csidrivers 2>/dev/null | head -10
echo ""
echo -e "${CYAN}CSI node plugins:${RESET}"
kubectl get pods -A -l "app.kubernetes.io/component=csi-driver" 2>/dev/null | head -10 || \
  kubectl get pods -n kube-system 2>/dev/null | grep -i "csi" | head -10

echo -e "\n${GREEN}Use --pvc <name> -n <namespace> for PVC deep-dive.${RESET}"
