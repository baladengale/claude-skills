#!/usr/bin/env bash
# k8s-rbac-audit — RBAC permissions audit and diagnostics
# Usage: diagnose.sh [--cluster-audit] [-n ns] [--sa <name>] [--can-i "<verb resource>"]

set -euo pipefail

NAMESPACE=""
SA=""
AUDIT_CLUSTER=false
CAN_I=""
AS_WHO=""

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    --cluster-audit) AUDIT_CLUSTER=true; shift ;;
    -n)              NAMESPACE="$2"; shift 2 ;;
    --sa)            SA="$2"; shift 2 ;;
    --can-i)         CAN_I="$2"; shift 2 ;;
    --as)            AS_WHO="$2"; shift 2 ;;
    -h|--help)       echo "Usage: $0 [--cluster-audit] [-n ns] [--sa name]"; exit 0 ;;
    *)               shift ;;
  esac
done

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║    K8s RBAC Audit — Permission Check     ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"

# ── Cluster-admin bindings ────────────────────────────────────────────────────
banner "cluster-admin Bindings (Security Critical)"
kubectl get clusterrolebindings -o json 2>/dev/null | \
  jq -r '.items[] |
  select(.roleRef.name == "cluster-admin") |
  "  [cluster-admin] " + .metadata.name + " → " +
  ([.subjects[]? | .kind + ":" + .name] | join(", "))' | \
  while read -r line; do echo -e "${RED}$line${RESET}"; done

# ── Wildcard permissions ───────────────────────────────────────────────────────
if $AUDIT_CLUSTER; then
  banner "Wildcard (*) Permissions in ClusterRoles"
  kubectl get clusterroles -o json 2>/dev/null | jq -r '
    .items[] |
    select(.metadata.name | startswith("system:") | not) |
    select(.rules[]? | (.verbs[] == "*") or (.resources[] == "*") or (.apiGroups[] == "*")) |
    "  ⚠ ClusterRole: " + .metadata.name' | \
    while read -r line; do echo -e "${YELLOW}$line${RESET}"; done

  banner "Non-system ClusterRoleBindings"
  kubectl get clusterrolebindings -o wide 2>/dev/null | \
    grep -v "^system:\|NAME" | head -30
fi

# ── ServiceAccount RBAC check ─────────────────────────────────────────────────
if [ -n "$SA" ]; then
  NS="${NAMESPACE:-default}"
  banner "ServiceAccount: $SA (ns: $NS)"

  echo -e "${CYAN}RoleBindings:${RESET}"
  kubectl get rolebindings -n "$NS" -o json 2>/dev/null | jq -r --arg sa "$SA" '
    .items[] |
    select(.subjects[]? | .kind == "ServiceAccount" and .name == $sa) |
    "  RoleBinding: " + .metadata.name + " → Role: " + .roleRef.name'

  echo -e "${CYAN}ClusterRoleBindings:${RESET}"
  kubectl get clusterrolebindings -o json 2>/dev/null | jq -r --arg sa "$SA" --arg ns "$NS" '
    .items[] |
    select(.subjects[]? | .kind == "ServiceAccount" and .name == $sa and .namespace == $ns) |
    "  ClusterRoleBinding: " + .metadata.name + " → ClusterRole: " + .roleRef.name'

  echo -e "\n${CYAN}Effective permissions (kubectl auth can-i --list):${RESET}"
  kubectl auth can-i --list \
    --as="system:serviceaccount:$NS:$SA" \
    -n "$NS" 2>/dev/null | head -20
fi

# ── Custom can-i check ────────────────────────────────────────────────────────
if [ -n "$CAN_I" ]; then
  NS_FLAG="${NAMESPACE:+-n $NAMESPACE}"
  AS_FLAG="${AS_WHO:+--as $AS_WHO}"
  banner "Permission Check: '$CAN_I'"
  # shellcheck disable=SC2086
  RESULT=$(kubectl auth can-i $CAN_I $NS_FLAG $AS_FLAG 2>/dev/null || echo "error")
  if [ "$RESULT" = "yes" ]; then
    echo -e "${GREEN}  ✓ ALLOWED: $CAN_I${RESET}"
  else
    echo -e "${RED}  ✗ DENIED: $CAN_I${RESET}"
  fi
fi

# ── Namespace RoleBindings summary ────────────────────────────────────────────
if [ -n "$NAMESPACE" ]; then
  banner "RoleBindings in namespace: $NAMESPACE"
  kubectl get rolebindings -n "$NAMESPACE" -o wide 2>/dev/null
fi

echo -e "\n${GREEN}Tips:${RESET}"
echo "  Check SA perms: $0 -n <ns> --sa <serviceaccount-name>"
echo "  Test access:    kubectl auth can-i get pods --as=system:serviceaccount:<ns>:<sa>"
echo "  List all perms: kubectl auth can-i --list --as=system:serviceaccount:<ns>:<sa>"
