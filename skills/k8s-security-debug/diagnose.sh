#!/usr/bin/env bash
# k8s-security-debug — Security posture and policy diagnostics
# Usage: diagnose.sh [--posture] [--gatekeeper] [--falco] [--all]

set -euo pipefail

CHECK_POSTURE=false
CHECK_GK=false
CHECK_FALCO=false
ALL=false

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

banner() { echo -e "\n${BOLD}${CYAN}── $1 ────────────────────────────────────────────────────────────${RESET}"; }

while [[ $# -gt 0 ]]; do
  case $1 in
    --posture)     CHECK_POSTURE=true; shift ;;
    --gatekeeper)  CHECK_GK=true; shift ;;
    --falco)       CHECK_FALCO=true; shift ;;
    --all)         ALL=true; CHECK_POSTURE=true; CHECK_GK=true; CHECK_FALCO=true; shift ;;
    -h|--help)     echo "Usage: $0 [--posture] [--gatekeeper] [--falco] [--all]"; exit 0 ;;
    *)             shift ;;
  esac
done

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${CYAN}║  K8s Security Debug — Posture Check      ║${RESET}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${RESET}"

# ── PSA namespace labels ──────────────────────────────────────────────────────
banner "Pod Security Admission (PSA) per Namespace"
kubectl get namespaces -o json 2>/dev/null | jq -r '
  .items[] |
  .metadata.name as $ns |
  (.metadata.labels | to_entries[] |
   select(.key | startswith("pod-security.kubernetes.io")) |
   $ns + " → " + .key + "=" + .value)
' | head -20 | while read -r line; do
  if echo "$line" | grep -q "restricted"; then
    echo -e "${GREEN}  ✓ $line${RESET}"
  elif echo "$line" | grep -q "baseline"; then
    echo -e "${CYAN}  ~ $line${RESET}"
  else
    echo "  $line"
  fi
done

# ── Privileged pods ───────────────────────────────────────────────────────────
if $CHECK_POSTURE || $ALL; then
  banner "Privileged / Root Containers (Security Risk)"
  kubectl get pods -A -o json 2>/dev/null | \
    jq -r '.items[] |
      select(
        .spec.hostNetwork == true or
        (.spec.containers[].securityContext.privileged == true) or
        (.spec.containers[].securityContext.runAsUser == 0)
      ) |
      "  ⚠ " + .metadata.namespace + "/" + .metadata.name' | \
    grep -v "kube-system\|openshift-" | \
    while read -r line; do echo -e "${YELLOW}$line${RESET}"; done || \
    echo -e "${GREEN}  No obvious privileged workloads in non-system namespaces${RESET}"
fi

# ── OPA Gatekeeper ────────────────────────────────────────────────────────────
if $CHECK_GK || $ALL; then
  banner "OPA Gatekeeper"
  if kubectl get pods -n gatekeeper-system --no-headers 2>/dev/null | grep -q "Running"; then
    echo -e "${GREEN}  Gatekeeper is installed${RESET}"
    echo -e "\n${CYAN}  Constraint violations:${RESET}"
    kubectl get constraints -o json 2>/dev/null | jq -r '
      .items[] |
      select(.status.totalViolations > 0) |
      "  ⚠ " + .metadata.name + ": " + (.status.totalViolations | tostring) + " violations"' | \
      while read -r line; do echo -e "${YELLOW}$line${RESET}"; done
  else
    echo -e "${YELLOW}  Gatekeeper not installed (optional)${RESET}"
  fi

  # Kyverno
  if kubectl get pods -n kyverno --no-headers 2>/dev/null | grep -q "Running"; then
    echo -e "\n${GREEN}  Kyverno is installed${RESET}"
    VIOLATIONS=$(kubectl get policyreport -A --no-headers 2>/dev/null | \
      awk '{sum+=$4} END {print sum}' || echo "0")
    echo -e "  Total policy violations: ${YELLOW}$VIOLATIONS${RESET}"
  fi
fi

# ── Falco ────────────────────────────────────────────────────────────────────
if $CHECK_FALCO || $ALL; then
  banner "Falco Runtime Security"
  if kubectl get pods -n falco --no-headers 2>/dev/null | grep -q "Running"; then
    echo -e "${GREEN}  Falco is running${RESET}"
    echo -e "${CYAN}  Recent alerts (last 10):${RESET}"
    kubectl logs -n falco daemonset/falco --tail=20 2>/dev/null | \
      grep -i "warning\|critical\|error\|alert" | tail -10 | \
      while read -r line; do echo -e "${YELLOW}  $line${RESET}"; done
  else
    echo -e "${YELLOW}  Falco not installed. Install: https://falco.org/docs/getting-started/kubernetes${RESET}"
  fi
fi

# ── cluster-admin bindings (always check) ────────────────────────────────────
banner "Overprivileged Bindings"
echo -e "${CYAN}cluster-admin subjects (non-system):${RESET}"
kubectl get clusterrolebindings -o json 2>/dev/null | jq -r '
  .items[] |
  select(.roleRef.name == "cluster-admin") |
  (.subjects // [])[] |
  select(.name | startswith("system:") | not) |
  "  ⚠ " + .kind + "/" + .name + " has cluster-admin"' | \
  while read -r line; do echo -e "${RED}$line${RESET}"; done

echo -e "\n${GREEN}Key tools:${RESET}"
echo "  kube-bench:  kubectl apply -f https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml"
echo "  trivy:       trivy k8s --report all --namespace production"
echo "  falco:       helm install falco falcosecurity/falco -n falco"
