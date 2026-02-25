---
name: kube-proxy-debug
description: kube-proxy troubleshooting - iptables vs ipvs mode, Service DNAT rules, conntrack table exhaustion, kube-proxy restarts, iptables rule verification, and debugging service routing at the kernel level.
metadata:
  emoji: "🔀"
  requires:
    bins: ["kubectl", "bash"]
---

# kube-proxy Debug — Service Routing Troubleshooting

kube-proxy is responsible for implementing Kubernetes Service routing on each node. It programs iptables (or ipvs) rules to DNAT ClusterIP/NodePort traffic to healthy pod endpoints.

Inspired by: Kubernetes kube-proxy docs, Tigera kube-proxy deep-dive, Cloudflare iptables blog, IPVS vs iptables comparison.

## When to Activate

Activate when the user asks about:
- kube-proxy not working, Service not routing
- iptables rules for Kubernetes services
- ipvs mode vs iptables mode kube-proxy
- conntrack table full, nf_conntrack overflow
- NodePort not working, external traffic not reaching pods
- Service routing broken after node restart
- kube-proxy logs, kube-proxy errors
- iptables DNAT rules for ClusterIP
- kube-proxy sync errors, endpoint updates slow

## Script Location

```
skills/kube-proxy-debug/diagnose.sh
```

## Usage

```bash
# kube-proxy health and mode check
bash skills/kube-proxy-debug/diagnose.sh

# Verify iptables rules for a specific service
bash skills/kube-proxy-debug/diagnose.sh --service my-svc -n production

# Check conntrack table
bash skills/kube-proxy-debug/diagnose.sh --conntrack
```

---

## Troubleshooting Runbook

### How kube-proxy Works

```
Client Pod → ClusterIP:Port
     ↓
iptables PREROUTING chain
     ↓
KUBE-SERVICES chain (kube-proxy managed)
     ↓
KUBE-SVC-<hash> chain → statistically load balances across endpoints
     ↓
KUBE-SEP-<hash> chain → DNAT to pod IP:port
     ↓
Target Pod
```

In **ipvs mode** the kernel handles load balancing (LVS) — faster for large clusters.

### Step 1 — kube-proxy Health

```bash
# kube-proxy DaemonSet (runs on every node)
kubectl get pods -n kube-system -l k8s-app=kube-proxy -o wide

# kube-proxy mode (iptables or ipvs)
kubectl get configmap kube-proxy -n kube-system -o yaml | grep mode

# kube-proxy logs (check for sync errors)
kubectl logs -n kube-system \
  $(kubectl get pod -n kube-system -l k8s-app=kube-proxy -o name | head -1) \
  --tail=30

# Errors to look for:
# "Failed to sync iptables rules" → iptables permission or lock issue
# "Dropping out of sync" → kube-proxy can't sync endpoints fast enough
# "panic" → kube-proxy crash
```

---

## Failure Mode: Service Not Routing (iptables mode)

**Diagnosis — verify iptables rules exist for a Service:**

```bash
# Access a node (via kubectl debug or SSH)
kubectl debug node/<node-name> -it --image=ubuntu

# Inside the node:
# Check if KUBE-SERVICES chain exists
iptables -L KUBE-SERVICES -n -v | head -20

# Find rules for a specific ClusterIP
CLUSTER_IP=$(kubectl get svc <svc-name> -n <ns> -o jsonpath='{.spec.clusterIP}')
iptables -L KUBE-SERVICES -n | grep "$CLUSTER_IP"

# Follow the chain for a service
iptables -L KUBE-SVC-<HASH> -n -v

# Check DNAT rules (endpoints)
iptables -L KUBE-SEP-<HASH> -n -v
# Should show: DNAT tcp -- anywhere anywhere to:<pod-ip>:<port>

# List all kube-proxy chains
iptables -L -n | grep "^Chain KUBE" | awk '{print $2}'

# Check NodePort rules
iptables -L KUBE-NODEPORTS -n -v
```

**If iptables rules are missing:**
```bash
# Restart kube-proxy to force sync
kubectl rollout restart daemonset kube-proxy -n kube-system

# Or delete the pod on a specific node
kubectl delete pod -n kube-system -l k8s-app=kube-proxy \
  --field-selector="spec.nodeName=<node-name>"
```

---

## Failure Mode: kube-proxy in ipvs Mode

```bash
# Check if ipvs is active
kubectl get configmap kube-proxy -n kube-system -o yaml | grep "mode: ipvs"

# On node — check IPVS virtual servers
ipvsadm -L -n

# Find virtual server for a ClusterIP
CLUSTER_IP=$(kubectl get svc <svc-name> -n <ns> -o jsonpath='{.spec.clusterIP}')
ipvsadm -L -n | grep -A5 "$CLUSTER_IP"

# Check kernel modules for ipvs
lsmod | grep -E "ip_vs|nf_conntrack"

# If ipvs modules missing:
modprobe ip_vs ip_vs_rr ip_vs_wrr ip_vs_sh nf_conntrack

# Check kube-proxy ipvs stats
ipvsadm -L --stats

# Flush and rebuild ipvs rules (emergency)
ipvsadm --clear
kubectl rollout restart daemonset kube-proxy -n kube-system
```

---

## Failure Mode: conntrack Table Exhaustion

**Symptom:** New connections to services start failing, SYN packets dropped, intermittent connectivity. Can happen under high load.

```bash
# Check conntrack table usage (on node)
cat /proc/sys/net/netfilter/nf_conntrack_count
cat /proc/sys/net/netfilter/nf_conntrack_max
# If count ≈ max → table is full → connections being dropped

# View conntrack table
conntrack -L 2>/dev/null | head -20
conntrack -L 2>/dev/null | wc -l   # current count

# Check kernel drop counters
cat /proc/net/stat/nf_conntrack | grep -v "entries"

# Check dmesg for conntrack overflow
dmesg | grep -i "nf_conntrack: table full\|conntrack"

# Increase conntrack table size (temporary)
sysctl -w net.netfilter.nf_conntrack_max=524288

# Permanent fix (survives reboots)
cat >> /etc/sysctl.d/99-conntrack.conf << EOF
net.netfilter.nf_conntrack_max=524288
net.netfilter.nf_conntrack_tcp_timeout_established=600
net.netfilter.nf_conntrack_tcp_timeout_close_wait=60
EOF
sysctl -p /etc/sysctl.d/99-conntrack.conf

# Check TIME_WAIT connections (they stay in conntrack)
ss -s | grep timewait
ss -tan state time-wait | wc -l

# Reduce TIME_WAIT hold time
sysctl -w net.ipv4.tcp_fin_timeout=15
sysctl -w net.netfilter.nf_conntrack_tcp_timeout_time_wait=15
```

---

## Failure Mode: NodePort Not Accessible

```bash
# Verify NodePort is set
kubectl get svc <svc-name> -n <ns> -o yaml | grep -E "type|nodePort|port"

# Check iptables NodePort rules (on node)
kubectl debug node/<node-name> -it --image=ubuntu
# chroot /host
iptables -L KUBE-NODEPORTS -n -v | grep <nodeport-number>

# Test NodePort from outside (replace node-ip)
curl http://<node-ip>:<nodeport>/

# Check if NodePort is blocked by security group / firewall
# For cloud providers: check security group allows 30000-32767/TCP

# NodePort range (default: 30000-32767)
kubectl get configmap kube-proxy -n kube-system -o yaml | grep nodePortAddresses
# Empty = listen on all interfaces (any IP of the node)

# externalTrafficPolicy: Local (only routes to pods on same node!)
kubectl get svc <name> -n <ns> -o jsonpath='{.spec.externalTrafficPolicy}'
# If "Local": request must hit a node that has a pod running on it
# If "Cluster" (default): any node can receive and route to any pod
```

---

## kube-proxy Replacement: eBPF (Cilium / Calico)

If the cluster uses Cilium or Calico in eBPF mode, kube-proxy may be disabled entirely:

```bash
# Check if kube-proxy is running at all
kubectl get pods -n kube-system -l k8s-app=kube-proxy
# Empty → kube-proxy is replaced

# Cilium: check kube-proxy replacement status
kubectl exec -n kube-system \
  $(kubectl get pod -n kube-system -l k8s-app=cilium -o name | head -1) \
  -- cilium status | grep KubeProxyReplacement

# Calico: check eBPF mode
calicoctl get felixconfiguration default -o yaml | grep bpfKubeProxyIptablesCleanupEnabled
```

---

## Quick iptables Reference for Kubernetes

```bash
# List ALL kube-proxy chains with packet counts
iptables -L -n -v | grep -E "^Chain KUBE|^[0-9]"

# Find a pod's backend rule
iptables -t nat -L -n | grep <pod-ip>

# Check masquerade rule (source NAT for egress)
iptables -t nat -L KUBE-POSTROUTING -n -v

# Flush kube-proxy rules (dangerous — services stop working until kube-proxy re-syncs)
# iptables -t nat -F KUBE-SERVICES
# Only do this if you want kube-proxy to rebuild from scratch

# Count rules per chain
iptables -t nat -L -n --line-numbers | grep "^Chain" | wc -l
```

---

## References

- [Kubernetes: kube-proxy](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-proxy/)
- [Kubernetes Services: Iptables mode](https://kubernetes.io/docs/concepts/services-networking/service/#proxy-mode-iptables)
- [IPVS-based load balancing](https://kubernetes.io/docs/concepts/services-networking/service/#proxy-mode-ipvs)
- [Cloudflare: Linux conntrack](https://blog.cloudflare.com/conntrack-tales-one-thousand-and-one-flows/)
- [Cilium kube-proxy replacement](https://docs.cilium.io/en/stable/network/kubernetes/kubeproxy-free/)
