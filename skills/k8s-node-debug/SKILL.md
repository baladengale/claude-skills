---
name: k8s-node-debug
description: Kubernetes node troubleshooting - NotReady nodes, kubelet failures, memory/disk/PID pressure, node drain and cordon procedures, DaemonSet issues, and node-level resource exhaustion.
metadata:
  emoji: "🖥️"
  requires:
    bins: ["kubectl", "bash"]
---

# K8s Node Debug — Node Troubleshooting Runbook

Node-level diagnostics for Kubernetes clusters. Covers NotReady conditions, kubelet issues, eviction thresholds, drain/cordon procedures, and host-level debugging.

Inspired by: Kubernetes node troubleshooting docs, kured (node reboot daemon), node-problem-detector, AWS/GKE/AKS node docs.

## When to Activate

Activate when the user asks about:
- Node NotReady, node not ready, node down
- MemoryPressure, DiskPressure, PIDPressure
- Kubelet not running, kubelet crashed
- Node drain, node cordon, maintenance window
- Pods not scheduling on a node
- Node CPU/memory exhausted, node overloaded
- Node eviction, eviction threshold
- DaemonSet pod not running on node
- Node reboot, node restart
- Cluster autoscaler, node pool scaling

## Script Location

```
skills/k8s-node-debug/diagnose.sh
```

## Usage

```bash
# Cluster-wide node health summary
bash skills/k8s-node-debug/diagnose.sh

# Deep-dive on specific node
bash skills/k8s-node-debug/diagnose.sh --node <node-name>

# Show resource allocation across all nodes
bash skills/k8s-node-debug/diagnose.sh --resources
```

---

## Troubleshooting Runbook

### Step 1 — Node Status Overview

```bash
# Node status and conditions
kubectl get nodes -o wide
kubectl get nodes -o custom-columns="NAME:.metadata.name,STATUS:.status.conditions[-1].type,REASON:.status.conditions[-1].reason,AGE:.metadata.creationTimestamp"

# Nodes with conditions (NotReady, Pressure)
kubectl get nodes | grep -v " Ready "

# Detailed node conditions
kubectl describe node <node-name>
# Key sections: Conditions, Capacity, Allocatable, Events
```

---

## Failure Mode: Node NotReady

**Symptom:** Node shows `NotReady` status

**Root causes:**
1. Kubelet process crashed or stopped
2. Network connectivity lost between node and API server
3. Node is under severe resource pressure
4. Container runtime (containerd/docker) is down
5. Node was manually cordoned

**Diagnosis:**

```bash
# Check node conditions
kubectl describe node <node> | grep -A20 "Conditions:"

# Check if it's just cordoned (SchedulingDisabled)
kubectl get node <node> -o jsonpath='{.spec.unschedulable}'

# Access node directly (cloud providers)
# AWS: SSM Session Manager
aws ssm start-session --target <instance-id>
# GKE: gcloud compute ssh
gcloud compute ssh <instance-name> --zone=<zone>

# Or use kubectl node debug (K8s 1.23+)
kubectl debug node/<node> -it --image=ubuntu

# Inside the node — check kubelet
systemctl status kubelet
journalctl -u kubelet --since "30min ago" | tail -50

# Check container runtime
systemctl status containerd
systemctl status docker   # if using docker

# Disk space
df -h
du -sh /var/lib/containerd/  # container images/layers
du -sh /var/log/

# Memory
free -h
cat /proc/meminfo | grep -E "MemTotal|MemFree|MemAvailable"
```

**Remediation:**
```bash
# Restart kubelet
systemctl restart kubelet

# Restart containerd
systemctl restart containerd

# Free disk space (prune unused images)
crictl rmi --prune               # containerd
docker system prune -f           # docker (if applicable)

# Clear evicted pods to free disk
kubectl get pods -A --field-selector=status.phase=Failed | grep Evicted | \
  awk '{print $1, $2}' | xargs -n2 kubectl delete pod -n
```

---

## Failure Mode: MemoryPressure

**Symptom:** Node has `MemoryPressure=True` condition; pods are evicted

**Understanding eviction:**
- Kubelet evicts pods when node memory drops below `eviction-hard` threshold (default: `memory.available<100Mi`)
- Eviction order: BestEffort → Burstable → Guaranteed (QoS class)
- `eviction-soft` = warning threshold, waits `eviction-soft-grace-period` before evicting

**Diagnosis:**

```bash
# Node memory stats
kubectl describe node <node> | grep -E "memory|Memory"

# Pods consuming most memory
kubectl top pods -n <namespace> --sort-by=memory

# Total memory requests vs allocatable
kubectl describe node <node> | grep -A20 "Allocated resources:"

# Find memory hogs
kubectl get pods -A -o json | \
  jq -r '.items[] | .metadata.namespace + " " + .metadata.name + " " + (.spec.containers[].resources.limits.memory // "unlimited")' | \
  sort -k3 -hr | head -20

# Node-level OOM events
kubectl describe node <node> | grep -i "oom\|evict"
dmesg | grep -i "oom killer"   # from inside the node
```

**Remediation:**
```bash
# Evict a low-priority pod gracefully (free memory fast)
kubectl drain <node> --ignore-daemonsets --delete-emptydir-data \
  --pod-selector='priority=low'

# Set limits on namespace with ResourceQuota
kubectl apply -f - <<EOF
apiVersion: v1
kind: ResourceQuota
metadata:
  name: memory-quota
  namespace: <ns>
spec:
  hard:
    limits.memory: 16Gi
    requests.memory: 8Gi
EOF

# Set default limits with LimitRange
kubectl apply -f - <<EOF
apiVersion: v1
kind: LimitRange
metadata:
  name: default-limits
  namespace: <ns>
spec:
  limits:
  - default:
      memory: 256Mi
      cpu: 250m
    defaultRequest:
      memory: 128Mi
      cpu: 100m
    type: Container
EOF
```

---

## Failure Mode: DiskPressure

**Symptom:** Node has `DiskPressure=True`; pod evictions due to ephemeral-storage

**Diagnosis:**

```bash
# Disk usage on node
kubectl debug node/<node> -it --image=ubuntu -- df -h

# Find large files (inside node debug shell)
du -sh /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/* 2>/dev/null | \
  sort -rh | head -10
du -sh /var/log/pods/*/* | sort -rh | head -10
du -sh /tmp/* | sort -rh | head -10

# Pods using most ephemeral storage
kubectl get pods -A \
  -o json | jq -r '.items[] | .metadata.namespace + " " + .metadata.name + " " + \
  (.status.containerStatuses[]?.allocatedResources?.["ephemeral-storage"] // "none")'
```

**Remediation:**
```bash
# Clean up unused container images (must run on node)
crictl rmi --prune

# Remove stopped containers
crictl rm $(crictl ps -a -q --state Exited)

# Rotate logs
logrotate -f /etc/logrotate.conf

# Clean old pod logs
find /var/log/pods -name "*.log" -mtime +7 -delete

# If node is managed — increase disk in cloud console
# EKS: modify the node group launch template
# GKE: update the node pool disk size
```

---

## Node Drain and Cordon Procedures

**Cordon** — mark node unschedulable (existing pods keep running)
**Drain** — cordon + gracefully evict all pods (for maintenance/decommission)

```bash
# Cordon: prevent new pods from scheduling
kubectl cordon <node>

# Drain: evict all pods (except DaemonSets)
kubectl drain <node> \
  --ignore-daemonsets \
  --delete-emptydir-data \
  --grace-period=60 \
  --timeout=300s

# Drain with pod disruption budget awareness (recommended)
kubectl drain <node> \
  --ignore-daemonsets \
  --delete-emptydir-data \
  --disable-eviction=false  # respects PodDisruptionBudget

# Force drain (skips PDB) — use only if stuck
kubectl drain <node> \
  --ignore-daemonsets \
  --delete-emptydir-data \
  --force \
  --grace-period=30

# Uncordon: re-enable scheduling
kubectl uncordon <node>

# Check which pods would be evicted (dry run)
kubectl drain <node> --ignore-daemonsets --dry-run
```

---

## Node Resource Allocation Analysis

```bash
# Requested vs allocatable per node
kubectl describe nodes | grep -A8 "Allocated resources:"

# Find nodes with < 10% CPU allocatable remaining
kubectl get nodes -o json | jq -r '
  .items[] |
  .metadata.name as $name |
  .status.allocatable.cpu as $cap |
  "Node: " + $name + " Allocatable CPU: " + $cap'

# Pods with no resource requests (dangerous — unscheduled QoS)
kubectl get pods -A -o json | \
  jq -r '.items[] | select(.spec.containers[].resources.requests == null) |
  .metadata.namespace + "/" + .metadata.name'

# All pod resource requests in a namespace
kubectl get pods -n <ns> \
  -o custom-columns="NAME:.metadata.name,CPU_REQ:.spec.containers[*].resources.requests.cpu,MEM_REQ:.spec.containers[*].resources.requests.memory,CPU_LIM:.spec.containers[*].resources.limits.cpu,MEM_LIM:.spec.containers[*].resources.limits.memory"
```

---

## DaemonSet Not Running on Node

```bash
# Check DaemonSet desired vs ready
kubectl get daemonset -n <ns>
kubectl describe daemonset <name> -n <ns>

# Check DaemonSet's nodeSelector / tolerations
kubectl get daemonset <name> -n <ns> -o jsonpath='{.spec.template.spec.nodeSelector}'
kubectl get daemonset <name> -n <ns> -o jsonpath='{.spec.template.spec.tolerations}'

# Compare with node labels/taints
kubectl get node <node> --show-labels
kubectl describe node <node> | grep Taints
```

---

## Cluster Autoscaler

```bash
# Check cluster autoscaler logs
kubectl logs -n kube-system deployment/cluster-autoscaler --tail=50

# Status
kubectl get configmap cluster-autoscaler-status -n kube-system -o yaml

# See why nodes can't scale down
kubectl logs -n kube-system deployment/cluster-autoscaler | grep "scale down"
```

---

## References

- [Kubernetes: Node Debugging](https://kubernetes.io/docs/tasks/debug/debug-cluster/)
- [Kubernetes: Kubelet Eviction](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/)
- [Node Problem Detector](https://github.com/kubernetes/node-problem-detector)
- [kured — Kubernetes Reboot Daemon](https://kured.dev/)
- [Cluster Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler)
