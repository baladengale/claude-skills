#!/usr/bin/env bash
# container-runtime-debug — Container runtime diagnostics via crictl
# Usage: diagnose.sh [--images] [--container <id>]
# Note: Run on the node or via: kubectl debug node/<node> -it --image=ubuntu

set -euo pipefail

CHECK_IMAGES=false
CONTAINER_ID=""

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    --images)     CHECK_IMAGES=true; shift ;;
    --container)  CONTAINER_ID="$2"; shift 2 ;;
    -h|--help)    echo "Usage: $0 [--images] [--container <id>]"; exit 0 ;;
    *)            shift ;;
  esac
done

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║ Container Runtime Debug — Node-Level     ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"

# ── Detect runtime ────────────────────────────────────────────────────────────
banner "Runtime Detection"
RUNTIME_ENDPOINT=""
for sock in /run/containerd/containerd.sock /var/run/crio/crio.sock /var/run/dockershim.sock; do
  if [ -S "$sock" ]; then
    echo -e "${GREEN}  ✓ Found runtime socket: $sock${RESET}"
    RUNTIME_ENDPOINT="unix://$sock"
    break
  fi
done

if [ -z "$RUNTIME_ENDPOINT" ]; then
  echo -e "${RED}  No container runtime socket found. Run this script on a node.${RESET}"
  echo "  Use: kubectl debug node/<node-name> -it --image=ubuntu"
  exit 1
fi

# ── Runtime health ────────────────────────────────────────────────────────────
banner "Runtime Service Status"
for svc in containerd crio docker; do
  if systemctl is-active "$svc" &>/dev/null; then
    echo -e "${GREEN}  ✓ $svc is active${RESET}"
  fi
done

# ── crictl checks ─────────────────────────────────────────────────────────────
if command -v crictl &>/dev/null; then
  export CONTAINER_RUNTIME_ENDPOINT="$RUNTIME_ENDPOINT"

  banner "Pod Sandboxes (crictl pods)"
  crictl pods 2>/dev/null | head -15

  banner "Non-Running Containers"
  crictl ps -a 2>/dev/null | grep -v "RUNNING\|CONTAINER" | head -15 | \
    while read -r line; do echo -e "${YELLOW}  $line${RESET}"; done

  if $CHECK_IMAGES; then
    banner "Cached Images"
    crictl images 2>/dev/null

    echo -e "\n${CYAN}Disk usage:${RESET}"
    df -h /var/lib/containerd 2>/dev/null || df -h /var/lib/containers 2>/dev/null || true
  fi

  if [ -n "$CONTAINER_ID" ]; then
    banner "Container Inspect: $CONTAINER_ID"
    crictl inspect "$CONTAINER_ID" 2>/dev/null | jq '{
      id: .status.id,
      state: .status.state,
      image: .status.image.image,
      reason: .status.reason,
      message: .status.message,
      exitCode: .status.exitCode
    }' 2>/dev/null

    echo -e "\n${CYAN}Container logs:${RESET}"
    crictl logs --tail=20 "$CONTAINER_ID" 2>/dev/null
  fi
else
  echo -e "${YELLOW}  crictl not found on this node.${RESET}"
  echo "  Install: https://github.com/kubernetes-sigs/cri-tools/releases"
fi

# ── Disk space ────────────────────────────────────────────────────────────────
banner "Disk Space"
df -h 2>/dev/null | grep -E "Filesystem|/var/lib|/$" | head -5

echo -e "\n${GREEN}Run on node via: kubectl debug node/<node> -it --image=ubuntu${RESET}"
