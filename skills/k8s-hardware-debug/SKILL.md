---
name: k8s-hardware-debug
description: Kubernetes hardware-level troubleshooting - GPU nodes with NVIDIA device plugin, NUMA topology, hugepages, CPU manager policy, hardware failures (disk, NIC, memory), SR-IOV, and node feature discovery.
metadata:
  emoji: "🖱️"
  requires:
    bins: ["kubectl", "bash"]
---

# K8s Hardware Debug — Hardware & Specialized Node Troubleshooting

Diagnostics for hardware-accelerated Kubernetes workloads: GPU scheduling (NVIDIA), NUMA awareness, hugepages, CPU Manager, SR-IOV networking, and hardware failure detection.

Inspired by: NVIDIA GPU Operator docs, Kubernetes Device Plugin framework, NUMA topology manager, Node Feature Discovery (NFD), Intel SR-IOV Network Operator.

## When to Activate

Activate when the user asks about:
- GPU not available in pod, CUDA error, nvidia-smi
- NVIDIA device plugin, GPU operator
- NUMA topology, CPU pinning, CPU Manager
- Hugepages, THP (Transparent Hugepages)
- SR-IOV, DPDK, high-performance networking
- Node Feature Discovery (NFD)
- Hardware failure, disk failure, NIC failure, memory error
- ECC memory errors, GPU thermal throttling
- CPU Manager policy, cpuset cgroup
- Accelerators: FPGA, TPU, Intel QAT

## Troubleshooting Runbook

### Step 1 — Hardware Node Inventory

```bash
# List nodes with their labels (shows hardware capabilities)
kubectl get nodes --show-labels | grep -E "accelerator\|nvidia\|gpu\|fpga"

# Extended resources per node (GPU, FPGA allocatable)
kubectl get nodes -o json | jq -r '
  .items[] |
  .metadata.name as $node |
  .status.allocatable |
  to_entries[] |
  select(.key | contains("nvidia") or contains("amd") or contains("fpga")) |
  $node + ": " + .key + " = " + .value'

# Node capacity including extended resources
kubectl describe node <node-name> | grep -A20 "Capacity:"
kubectl describe node <node-name> | grep -A20 "Allocatable:"
```

---

## Failure Mode: GPU Not Available / CUDA Errors

**Symptoms:** Pod stays pending with `Insufficient nvidia.com/gpu`, or pod runs but CUDA fails

### Checking GPU Node Health

```bash
# Check if NVIDIA device plugin is running (DaemonSet on GPU nodes)
kubectl get pods -n kube-system | grep nvidia
kubectl get daemonset -n kube-system nvidia-device-plugin-daemonset

# Device plugin logs (shows GPU discovery)
kubectl logs -n kube-system \
  $(kubectl get pod -n kube-system -l name=nvidia-device-plugin-ds -o name | head -1) \
  --tail=30

# GPU Operator components
kubectl get pods -n gpu-operator
kubectl get pods -n gpu-operator-resources

# Which nodes have GPUs registered
kubectl get nodes -o json | jq -r '
  .items[] |
  select(.status.allocatable | has("nvidia.com/gpu")) |
  .metadata.name + ": " + .status.allocatable["nvidia.com/gpu"] + " GPU(s)"'

# Which GPUs are currently allocated to pods
kubectl get pods -A -o json | jq -r '
  .items[] |
  select(.spec.containers[].resources.limits["nvidia.com/gpu"] != null) |
  .metadata.namespace + "/" + .metadata.name + ": " +
  (.spec.containers[].resources.limits["nvidia.com/gpu"] // "0") + " GPU(s)"'

# Run nvidia-smi on a GPU node
kubectl debug node/<gpu-node> -it --image=nvidia/cuda:11.8-base-ubuntu22.04 -- \
  nvidia-smi

# Or from inside the device plugin pod
kubectl exec -n kube-system \
  $(kubectl get pod -n kube-system -l name=nvidia-device-plugin-ds \
    --field-selector="spec.nodeName=<gpu-node>" -o name) \
  -- nvidia-smi
```

---

## GPU Workload Debugging

```bash
# Check GPU resource request in pod spec
kubectl get pod <pod> -n <ns> \
  -o jsonpath='{.spec.containers[*].resources.limits}'

# Correct GPU request syntax:
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: gpu-app
    image: nvidia/cuda:11.8-runtime-ubuntu22.04
    resources:
      limits:
        nvidia.com/gpu: 1      # request 1 GPU
      requests:
        nvidia.com/gpu: 1
  nodeSelector:
    accelerator: nvidia-tesla-v100  # optional: specific GPU type
  tolerations:                       # if GPU nodes are tainted
  - key: nvidia.com/gpu
    operator: Exists
    effect: NoSchedule
EOF

# Test GPU access inside pod
kubectl exec <pod> -n <ns> -- nvidia-smi
kubectl exec <pod> -n <ns> -- python3 -c "import torch; print(torch.cuda.is_available())"

# GPU thermal throttling / power limits
kubectl exec <pod> -n <ns> -- nvidia-smi --query-gpu=temperature.gpu,power.draw,clocks.sm --format=csv

# GPU utilization
kubectl exec <pod> -n <ns> -- nvidia-smi dmon -s u -d 1

# Check ECC memory errors
kubectl exec <pod> -n <ns> -- nvidia-smi --query-gpu=ecc.errors.corrected.volatile.total,ecc.errors.uncorrected.volatile.total --format=csv

# MIG (Multi-Instance GPU) configuration
kubectl exec <pod> -n <ns> -- nvidia-smi mig -lgip
kubectl exec <pod> -n <ns> -- nvidia-smi mig -lgi
```

---

## NUMA Topology Manager

**NUMA (Non-Uniform Memory Access)** — for high-performance workloads (AI/ML, HFT, NFV) where memory locality matters.

```bash
# Check Topology Manager policy
cat /var/lib/kubelet/config.yaml | grep -A3 "topologyManager"
# Policies:
# none (default) → no NUMA awareness
# best-effort → prefer NUMA, but schedule anyway if not possible
# restricted → require NUMA alignment for guaranteed QoS pods
# single-numa-node → require single NUMA node

# Node NUMA topology (on the node)
kubectl debug node/<node> -it --image=ubuntu
# chroot /host
# numactl --hardware
# lscpu | grep NUMA

# Check CPU Manager policy
cat /var/lib/kubelet/config.yaml | grep cpuManager
# none (default) → no CPU pinning
# static → pin CPUs for guaranteed QoS pods with integer CPU requests

# Verify CPU pinning is working (inside the pod)
kubectl exec <pod> -n <ns> -- cat /proc/self/status | grep Cpus_allowed
# OR
kubectl exec <pod> -n <ns> -- taskset -cp 1  # shows which CPUs process 1 can use

# Resource Topology Exporter (for NUMA-aware scheduling)
kubectl get pods -n node-feature-discovery
```

---

## Hugepages

Hugepages improve performance for memory-intensive workloads by using larger memory pages (2Mi or 1Gi instead of 4Ki).

```bash
# Check hugepages configured on node
kubectl describe node <node> | grep -E "hugepages|Hugepages"

# Node allocatable hugepages
kubectl get node <node> -o json | jq -r '
  .status.allocatable |
  to_entries[] |
  select(.key | contains("hugepages")) |
  .key + " = " + .value'

# On the node:
kubectl debug node/<node> -it --image=ubuntu
# chroot /host
# cat /proc/meminfo | grep -i huge
# HugePages_Total: 128
# HugePages_Free:  128
# Hugepagesize:    2048 kB

# Configure hugepages on node (at boot via kernel args or sysctl)
# echo 128 > /proc/sys/vm/nr_hugepages  (temporary)
# Or via MachineConfig / kubelet args: --feature-gates=HugePages=true (enabled by default)

# Pod requesting hugepages:
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: app
    resources:
      requests:
        hugepages-2Mi: 256Mi     # request 128 × 2Mi hugepages
        memory: 256Mi            # memory request must equal hugepages total
      limits:
        hugepages-2Mi: 256Mi
        memory: 256Mi
    volumeMounts:
    - mountPath: /hugepages-2Mi
      name: hugepage-2mi
  volumes:
  - name: hugepage-2mi
    emptyDir:
      medium: HugePages-2Mi
EOF

# Transparent Huge Pages (THP) — opposite of pinned hugepages
# THP can cause latency spikes for databases (Redis, MongoDB recommend disabling)
# Check THP status on node:
cat /sys/kernel/mm/transparent_hugepage/enabled
# [always] = THP enabled (bad for DBs)
# [madvise] = only when madvise'd
# [never] = disabled (recommended for latency-sensitive)

# Disable THP (MachineConfig or node debug):
echo never > /sys/kernel/mm/transparent_hugepage/enabled
```

---

## Hardware Failure Detection

```bash
# Check for hardware errors in kernel logs (on node)
kubectl debug node/<node> -it --image=ubuntu
# chroot /host
# dmesg | grep -iE "error|fail|hardware|corrected|uncorrected|machine check"
# journalctl -k | grep -iE "ECC|MCE|hardware error|disk error"

# Machine Check Exceptions (MCE) — CPU/memory hardware errors
# dmesg | grep -i "mce\|machine check"
# mcelog --client  (if mcelog is installed)

# Disk errors
# dmesg | grep -iE "I/O error|disk error|ata.*error|scsi.*error"
# smartctl -a /dev/sda  (SMART data for disk health)

# NIC errors
# ip -s link show eth0
# ethtool -S eth0 | grep -i error

# Memory errors (hardware ECC)
# edac-util -s 4  (EDAC error checking)
# dmidecode --type 17  (memory info)

# Node condition for hardware issues
kubectl describe node <node> | grep -A5 "Conditions:"
# MemoryPressure, DiskPressure, PIDPressure

# NPD (Node Problem Detector) — Kubernetes-native hardware monitoring
kubectl get pods -n kube-system | grep node-problem-detector
kubectl logs -n kube-system <npd-pod> --tail=20

# NPD conditions (custom node conditions set by NPD)
kubectl describe node <node> | grep -i "condition\|nfd\|problem"
```

---

## Node Feature Discovery (NFD)

NFD auto-labels nodes with hardware capabilities (CPU flags, NUMA, GPU, SR-IOV, FPGA).

```bash
# NFD pods
kubectl get pods -n node-feature-discovery

# Labels added by NFD
kubectl get node <node> --show-labels | tr ',' '\n' | grep "feature.node.kubernetes.io"

# Common NFD labels:
# feature.node.kubernetes.io/cpu-cpuid.AVX512F=true
# feature.node.kubernetes.io/kernel-config.NO_HZ_FULL=true
# feature.node.kubernetes.io/memory-numa=true
# feature.node.kubernetes.io/network.sriov.capable=true
# feature.node.kubernetes.io/pci-0300_10de.present=true  (NVIDIA GPU)

# Schedule workloads on nodes with specific features
kubectl apply -f - <<EOF
spec:
  nodeSelector:
    feature.node.kubernetes.io/cpu-cpuid.AVX512F: "true"  # requires AVX512
    feature.node.kubernetes.io/memory-numa: "true"         # NUMA nodes available
EOF
```

---

## SR-IOV (High-Performance Network)

SR-IOV allows pods to directly access NIC hardware for ultra-low latency networking (NFV, 5G, HFT).

```bash
# SR-IOV Network Operator
kubectl get pods -n sriov-network-operator

# SR-IOV node status
kubectl get sriovnetworknodestates -A

# SR-IOV policies
kubectl get sriovnetworknodepolicies -A

# SR-IOV networks
kubectl get sriovnetworks -A

# Check VF (Virtual Function) allocation on node
kubectl debug node/<node> -it --image=ubuntu
# chroot /host
# cat /sys/class/net/eth0/device/sriov_numvfs
# ip link show | grep "vf "
```

---

## References

- [NVIDIA GPU Operator](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/)
- [Kubernetes: Device Plugins](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/)
- [Kubernetes: CPU Manager](https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/)
- [Kubernetes: Topology Manager](https://kubernetes.io/docs/tasks/administer-cluster/topology-manager/)
- [Kubernetes: Hugepages](https://kubernetes.io/docs/tasks/manage-hugepages/scheduling-hugepages/)
- [Node Feature Discovery](https://kubernetes-sigs.github.io/node-feature-discovery/)
- [SR-IOV Network Operator](https://github.com/k8snetworkplumbingwg/sriov-network-operator)
