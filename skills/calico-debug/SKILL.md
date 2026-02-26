---
name: calico-debug
description: Calico CNI and network policy diagnostics - Felix agent issues, BGP peering, BIRD routing, IP pool exhaustion, GlobalNetworkPolicy troubleshooting, calicoctl commands, and dataplane verification.
metadata:
  emoji: "🐦"
  requires:
    bins: ["kubectl", "bash"]
---

# Calico Debug — CNI and Network Policy Troubleshooting

Systematic Calico diagnostics covering Felix, BGP/BIRD, IP pool management, network policy enforcement, and dataplane (iptables/eBPF) verification.

Inspired by: Tigera/Calico official docs, calicoctl reference, Calico troubleshooting guide, Calico eBPF dataplane docs.

## When to Activate

Activate when the user asks about:
- Calico not working, Felix agent down
- BGP peering failed, BGP routes not propagating
- BIRD routing daemon issues
- Calico IP pool exhaustion, pod can't get IP
- NetworkPolicy not enforcing, Calico policy bypass
- GlobalNetworkPolicy, GlobalNetworkSet
- calicoctl commands, Calico resources
- Calico eBPF dataplane, eBPF mode
- Calico node not ready, IPAM issues
- WireGuard encryption, Calico encryption

## Script Location

```
skills/calico-debug/diagnose.sh
```

## Usage

```bash
# Calico component health
bash skills/calico-debug/diagnose.sh --health

# BGP peer status
bash skills/calico-debug/diagnose.sh --bgp

# IP pool and IPAM status
bash skills/calico-debug/diagnose.sh --ipam

# Check policies for a pod
bash skills/calico-debug/diagnose.sh --pod my-pod -n production
```

---

## Troubleshooting Runbook

### Calico Architecture

```
┌─────────────────────────────────────────────┐
│  Calico Components                          │
│                                             │
│  calico-node (DaemonSet on every node)      │
│  ├── Felix    → programs iptables/eBPF      │
│  └── BIRD     → BGP route distribution      │
│                                             │
│  calico-kube-controllers (Deployment)       │
│  └── syncs K8s resources → Calico           │
│                                             │
│  calico-typha (optional, for large clusters)│
│  └── caches Felix connections               │
│                                             │
│  calicoctl  → CLI for Calico resources      │
└─────────────────────────────────────────────┘
```

### Step 1 — Calico Health Overview

```bash
# Calico node pods (must be Running on EVERY node)
kubectl get pods -n kube-system -l k8s-app=calico-node -o wide

# Calico kube-controllers
kubectl get pods -n kube-system -l k8s-app=calico-kube-controllers

# Calico Typha (large clusters)
kubectl get pods -n kube-system -l k8s-app=calico-typha

# calicoctl node status (run on a node or via kubectl exec)
kubectl exec -n kube-system \
  $(kubectl get pod -n kube-system -l k8s-app=calico-node -o name | head -1) \
  -- calico-node -bird-live

# Or if calicoctl is installed locally:
calicoctl node status
```

---

## Failure Mode: Felix Agent Not Working

**Felix** programs the dataplane (iptables or eBPF). If Felix is broken, network policies don't apply and pod networking may break.

```bash
# Check Felix logs
kubectl logs -n kube-system \
  $(kubectl get pod -n kube-system -l k8s-app=calico-node -o name | head -1) \
  -c calico-node --tail=50

# Look for:
# "Failed to read..." → API connectivity issue
# "Failed to apply policy" → dataplane error
# "Felix panicked" → restart Felix

# Felix diagnostics (inside calico-node pod)
kubectl exec -n kube-system <calico-node-pod> -- calico-node -felix-live

# Check Felix config
kubectl get felixconfiguration default -o yaml
calicoctl get felixconfiguration default -o yaml

# Felix important settings:
# spec.logSeverityScreen: Info  (change to Debug for more detail)
# spec.iptablesRefreshInterval: 90s  (how often iptables are resynced)
# spec.bpfEnabled: false  (false = iptables mode, true = eBPF mode)
```

**Enable Felix debug logging (temporary):**
```bash
calicoctl patch felixconfiguration default \
  --patch='{"spec":{"logSeverityScreen":"Debug"}}'
# Check logs then revert:
calicoctl patch felixconfiguration default \
  --patch='{"spec":{"logSeverityScreen":"Info"}}'
```

---

## Failure Mode: BGP Peering Issues

**Symptom:** Pods can reach services on same node but not cross-node. BGP routes not distributed.

```bash
# BGP peer status (via BIRD inside calico-node pod)
kubectl exec -n kube-system <calico-node-pod> -- birdcl show protocols all

# Expected output for healthy BGP:
# BGP_NODE_MESH  BGP       master   up     active  Established

# Check configured BGP peers
calicoctl get bgppeer
calicoctl get bgpconfig

# Show routes BIRD knows about
kubectl exec -n kube-system <calico-node-pod> -- birdcl show route

# BGP summary across all nodes
kubectl exec -n kube-system <calico-node-pod> -- birdcl show protocols

# Check if node-to-node mesh is enabled (default mode)
calicoctl get bgpconfig default -o yaml
# spec.nodeToNodeMeshEnabled: true  → full-mesh BGP (good for < 100 nodes)
# spec.nodeToNodeMeshEnabled: false → route reflector mode

# BGP troubleshooting with calicoctl
calicoctl node checksystem

# BIRD log inside calico-node pod
kubectl exec -n kube-system <calico-node-pod> -- \
  tail -100 /var/log/calico/bird/current
```

**Common BGP issues:**
- Port 179 blocked by firewall between nodes → BGP can't establish
- AS number mismatch → BGP peer rejects connection
- Node IP wrong → BGP peer can't connect

---

## Failure Mode: IP Pool Exhaustion / Pod Can't Get IP

**Symptom:** `kubectl describe pod` shows `failed to allocate IP address`, pod stuck in ContainerCreating

```bash
# List IP pools
calicoctl get ippool -o wide

# Check IPAM block allocations
calicoctl ipam show --show-blocks

# Check per-node allocation
calicoctl ipam show --show-borrowed

# Specific pool usage
calicoctl ipam show --ip=10.244.0.0/16

# Find leaked IPs (allocated but no pod using them)
calicoctl ipam check

# Check if any nodes are exhausted
calicoctl ipam show --show-blocks | grep -E "Block|In use"

# Release leaked/stale IPs
calicoctl ipam release --ip=10.244.x.x

# Add more IPs by creating a new IP pool
calicoctl apply -f - <<EOF
apiVersion: projectcalico.org/v3
kind: IPPool
metadata:
  name: additional-pool
spec:
  cidr: 10.245.0.0/16
  ipipMode: Always
  natOutgoing: true
  disabled: false
EOF
```

---

## Network Policy Enforcement

```bash
# List all Calico NetworkPolicies (namespace-scoped)
calicoctl get networkpolicy -A
calicoctl get networkpolicy -n <namespace>

# List GlobalNetworkPolicies (cluster-wide)
calicoctl get globalnetworkpolicy

# GlobalNetworkSets (IP-based groupings)
calicoctl get globalnetworkset

# NetworkSets (namespace-scoped)
calicoctl get networkset -n <namespace>

# Trace policy for a specific endpoint
# Calico 3.x provides policy-based tracing
calicoctl get workloadendpoint -n <namespace>

# Check what iptables rules Calico has installed (on node)
iptables -L cali-FORWARD -v -n | head -30
iptables -L cali-pro-<policy-name> -v -n

# In eBPF mode: check eBPF maps
kubectl exec -n kube-system <calico-node-pod> -- bpftool map list

# Policy advisor — simulate which policies apply
calicoctl get workloadendpoint <pod-name> -n <namespace> -o yaml
```

**Common GlobalNetworkPolicy pattern (deny all, allow egress DNS):**
```yaml
apiVersion: projectcalico.org/v3
kind: GlobalNetworkPolicy
metadata:
  name: default-deny
spec:
  selector: all()
  order: 100
  types:
  - Ingress
  - Egress
  egress:
  - action: Allow
    protocol: UDP
    destination:
      ports: [53]
  - action: Allow
    protocol: TCP
    destination:
      ports: [53]
```

---

## Calico eBPF Dataplane

eBPF mode replaces iptables with eBPF programs — better performance, faster policy updates.

```bash
# Check if eBPF mode is enabled
calicoctl get felixconfiguration default -o yaml | grep bpfEnabled

# Enable eBPF mode
calicoctl patch felixconfiguration default \
  --patch='{"spec":{"bpfEnabled":true}}'

# Verify eBPF programs loaded
kubectl exec -n kube-system <calico-node-pod> -- bpftool prog list | grep calico

# Check eBPF stats
kubectl exec -n kube-system <calico-node-pod> -- calico-node -bpf counters

# eBPF connect-time load balancing (replaces kube-proxy)
calicoctl get felixconfiguration default -o yaml | grep bpfKubeProxyIptablesCleanupEnabled
```

---

## WireGuard Encryption

```bash
# Check if WireGuard is enabled
calicoctl get felixconfiguration default -o yaml | grep wireguard

# Enable WireGuard
calicoctl patch felixconfiguration default \
  --patch='{"spec":{"wireguardEnabled":true}}'

# Verify WireGuard interface on nodes
kubectl exec -n kube-system <calico-node-pod> -- wg show

# Check WireGuard stats
calicoctl node status | grep WireGuard
```

---

## Quick calicoctl Reference

```bash
# Install calicoctl
curl -L https://github.com/projectcalico/calico/releases/latest/download/calicoctl-linux-amd64 -o calicoctl
chmod +x calicoctl && sudo mv calicoctl /usr/local/bin/

# Or run via kubectl exec:
kubectl exec -n kube-system \
  $(kubectl get pod -n kube-system -l k8s-app=calico-node -o name | head -1) \
  -- calicoctl <command>

# Key commands:
calicoctl node status           # BGP + Felix status
calicoctl get ippool -o wide    # IP pools
calicoctl ipam show             # IP allocation
calicoctl get networkpolicy -A  # network policies
calicoctl get globalnetworkpolicy   # cluster-wide policies
calicoctl get felixconfiguration    # Felix config
calicoctl get bgppeer               # BGP peers
```

---

## References

- [Calico Troubleshooting](https://docs.tigera.io/calico/latest/operations/troubleshoot/)
- [calicoctl reference](https://docs.tigera.io/calico/latest/reference/calicoctl/)
- [Calico eBPF dataplane](https://docs.tigera.io/calico/latest/operations/ebpf/)
- [Calico BGP](https://docs.tigera.io/calico/latest/networking/bgp)
- [Tigera Academy (free Calico training)](https://academy.tigera.io/)
