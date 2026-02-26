#!/usr/bin/env bash
# k8s-hardware-debug — GPU, NUMA, hugepages, hardware diagnostics
# Usage: diagnose.sh [--gpu] [--numa] [--hugepages] [--all]

set -euo pipefail

CHECK_GPU=false
CHECK_NUMA=false
CHECK_HUGEPAGES=false
ALL=false

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    --gpu)       CHECK_GPU=true; shift ;;
    --numa)      CHECK_NUMA=true; shift ;;
    --hugepages) CHECK_HUGEPAGES=true; shift ;;
    --all)       ALL=true; CHECK_GPU=true; CHECK_NUMA=true; CHECK_HUGEPAGES=true; shift ;;
    -h|--help)   echo "Usage: $0 [--gpu] [--numa] [--hugepages] [--all]"; exit 0 ;;
    *)           shift ;;
  esac
done

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║  K8s Hardware Debug — Accelerator Check  ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"

# ── Extended resources on all nodes ──────────────────────────────────────────
banner "Extended Resources (GPU, FPGA, SR-IOV)"
kubectl get nodes -o json 2>/dev/null | jq -r '
  .items[] |
  .metadata.name as $node |
  .status.allocatable |
  to_entries[] |
  select(.key | (contains("nvidia") or contains("amd.com") or contains("fpga") or contains("sriov") or contains("hugepages"))) |
  "  " + $node + ": " + .key + " = " + .value' | \
  while read -r line; do
    if echo "$line" | grep -q "nvidia\|amd\|fpga"; then
      echo -e "${GREEN}$line${RESET}"
    else
      echo -e "${CYAN}$line${RESET}"
    fi
  done

# ── GPU device plugin ─────────────────────────────────────────────────────────
if $CHECK_GPU || $ALL; then
  banner "NVIDIA Device Plugin"
  GPU_DS=$(kubectl get daemonset -A --no-headers 2>/dev/null | grep -i "nvidia\|gpu-operator" | head -3)
  if [ -n "$GPU_DS" ]; then
    echo -e "${GREEN}  GPU DaemonSet found:${RESET}"
    echo "$GPU_DS" | while read -r line; do echo "  $line"; done
  else
    echo -e "${YELLOW}  No NVIDIA device plugin DaemonSet found${RESET}"
    echo "  Install NVIDIA GPU Operator: https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/"
  fi

  echo -e "\n${CYAN}Pods using GPUs:${RESET}"
  kubectl get pods -A -o json 2>/dev/null | jq -r '
    .items[] |
    select(
      any(.spec.containers[].resources.limits // {} | keys[];
          startswith("nvidia") or startswith("amd.com"))
    ) |
    "  " + .metadata.namespace + "/" + .metadata.name' | head -10
fi

# ── Hugepages ─────────────────────────────────────────────────────────────────
if $CHECK_HUGEPAGES || $ALL; then
  banner "Hugepages"
  kubectl get nodes -o json 2>/dev/null | jq -r '
    .items[] |
    .metadata.name as $n |
    .status.allocatable |
    to_entries[] |
    select(.key | startswith("hugepages")) |
    "  " + $n + ": " + .key + " = " + .value' | \
    while read -r line; do
      if echo "$line" | grep -qE "0$|= 0$"; then
        echo -e "${YELLOW}$line (not configured)${RESET}"
      else
        echo -e "${GREEN}$line${RESET}"
      fi
    done
fi

# ── NUMA ──────────────────────────────────────────────────────────────────────
if $CHECK_NUMA || $ALL; then
  banner "NUMA / Topology Manager"
  echo -e "${CYAN}  Topology Manager Policy (check kubelet config on nodes):${RESET}"
  echo "  kubectl debug node/<node> -it --image=ubuntu"
  echo "  chroot /host && cat /var/lib/kubelet/config.yaml | grep -A3 topologyManager"
  echo ""

  echo -e "${CYAN}  NFD (Node Feature Discovery) labels for NUMA:${RESET}"
  kubectl get nodes -o json 2>/dev/null | jq -r '
    .items[] |
    .metadata.name as $n |
    (.metadata.labels | to_entries[] |
     select(.key | contains("numa") or contains("nfd") or contains("cpuid")) |
     "  " + $n + ": " + .key + "=" + .value)' | head -10
fi

# ── SR-IOV ────────────────────────────────────────────────────────────────────
banner "SR-IOV Network Operator"
if kubectl api-resources 2>/dev/null | grep -q "sriovnetwork"; then
  echo -e "${GREEN}  SR-IOV CRDs installed${RESET}"
  kubectl get sriovnetworknodestates -A 2>/dev/null | head -5
else
  echo -e "${YELLOW}  SR-IOV Network Operator not installed (optional)${RESET}"
fi

# ── Node Problem Detector ─────────────────────────────────────────────────────
banner "Node Problem Detector"
NPD=$(kubectl get pods -n kube-system -l k8s-app=node-problem-detector --no-headers 2>/dev/null | wc -l)
if [ "$NPD" -gt 0 ]; then
  echo -e "${GREEN}  Node Problem Detector is running ($NPD pods)${RESET}"
else
  echo -e "${YELLOW}  Node Problem Detector not installed${RESET}"
  echo "  Install for hardware failure detection: https://github.com/kubernetes/node-problem-detector"
fi

echo -e "\n${GREEN}Key commands:${RESET}"
echo "  GPU nodes:   kubectl get nodes -l accelerator=nvidia"
echo "  nvidia-smi:  kubectl debug node/<gpu-node> -it --image=nvidia/cuda:11.8-base-ubuntu22.04 -- nvidia-smi"
echo "  HW errors:   kubectl debug node/<node> -it --image=ubuntu → chroot /host → dmesg | grep -i error"
