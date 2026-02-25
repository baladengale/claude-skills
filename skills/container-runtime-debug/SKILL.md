---
name: container-runtime-debug
description: Container runtime diagnostics - containerd, CRI-O, and Docker troubleshooting using crictl, image pull failures, container stuck states, snapshotter issues, runtime crashes, and OCI spec problems.
metadata:
  emoji: "🐳"
  requires:
    bins: ["crictl", "bash"]
---

# Container Runtime Debug — Runtime Troubleshooting Runbook

Container runtime diagnostics for containerd, CRI-O, and Docker. Covers runtime crashes, image issues, container state problems, and CRI (Container Runtime Interface) debugging using `crictl`.

Inspired by: containerd docs, CRI-O troubleshooting guide, crictl reference, Kubernetes CRI docs, Docker troubleshooting guide.

## When to Activate

Activate when the user asks about:
- Container runtime not responding, kubelet lost contact with runtime
- containerd crashed, containerd.service failed
- CRI-O errors, CRI-O pod creation failure
- crictl commands, inspect containers on node
- Container stuck in created state, not starting
- Image pull failure at runtime level (not Kubernetes)
- Snapshotter error, overlay filesystem issue
- OCI runtime error, runc, crun failures
- Docker socket, dockershim (deprecated)
- Container image layers, image garbage collection

## Script Location

```
skills/container-runtime-debug/diagnose.sh
```

## Usage

```bash
# Runtime health and container status on current node
bash skills/container-runtime-debug/diagnose.sh

# Image inspection and cleanup
bash skills/container-runtime-debug/diagnose.sh --images

# Specific container deep-dive
bash skills/container-runtime-debug/diagnose.sh --container <id>
```

---

## Troubleshooting Runbook

### Architecture: Container Runtime Stack

```
kubelet
  ↓ CRI gRPC
Container Runtime (containerd / CRI-O)
  ↓ OCI spec
OCI Runtime (runc / crun / kata-runtime)
  ↓ syscalls
Linux Kernel (namespaces, cgroups, seccomp)
```

### Accessing the Node

Most runtime debugging requires access to the node itself:

```bash
# kubectl node debug (K8s 1.23+) — no SSH needed
kubectl debug node/<node-name> -it --image=ubuntu

# Inside the debug pod — access host filesystem via /host
chroot /host bash

# Access containerd socket
crictl --runtime-endpoint=unix:///run/containerd/containerd.sock ps

# Or set in environment
export CONTAINER_RUNTIME_ENDPOINT=unix:///run/containerd/containerd.sock
export IMAGE_SERVICE_ENDPOINT=unix:///run/containerd/containerd.sock
```

---

## Failure Mode: containerd Not Responding

**Symptom:** kubelet logs show `failed to connect to containerd`, all pods stuck in ContainerCreating

```bash
# Check containerd service (on node)
systemctl status containerd
journalctl -u containerd --since "30min ago" | tail -50

# Restart containerd
systemctl restart containerd

# Check containerd config
cat /etc/containerd/config.toml

# containerd health check
ctr version   # ctr is containerd's own CLI

# Check for containerd socket
ls -la /run/containerd/containerd.sock

# containerd log level (for debugging, edit /etc/containerd/config.toml)
# [debug]
#   level = "debug"
# Then: systemctl restart containerd

# Check for zombie containerd processes
ps aux | grep containerd | grep -v grep

# Common cause: disk full (containerd stores images + containers on disk)
df -h /var/lib/containerd

# Kill zombie and restart
pkill -f containerd
systemctl start containerd
```

---

## Failure Mode: CRI-O Not Responding

```bash
# CRI-O service status
systemctl status crio
journalctl -u crio --since "30min ago" | tail -50

# Restart CRI-O
systemctl restart crio

# CRI-O config
cat /etc/crio/crio.conf

# CRI-O version
crio --version

# Check CRI-O socket
ls -la /var/run/crio/crio.sock

# CRI-O storage (separate from containerd)
du -sh /var/lib/containers/storage
```

---

## crictl — The Universal CRI Debugging Tool

`crictl` works with ANY CRI-compliant runtime (containerd, CRI-O).

```bash
# Set endpoint (or via /etc/crictl.yaml)
export CONTAINER_RUNTIME_ENDPOINT=unix:///run/containerd/containerd.sock
# OR
cat > /etc/crictl.yaml << EOF
runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 30
debug: false
EOF

# List running pods (sandbox containers)
crictl pods
crictl pods --state Ready
crictl pods --name <pod-name>

# List containers
crictl ps         # running
crictl ps -a      # all (including stopped)

# Get container logs
crictl logs <container-id>
crictl logs --tail=50 <container-id>

# Inspect pod sandbox
crictl inspectp <pod-id>

# Inspect container
crictl inspect <container-id>

# Execute command in container
crictl exec -it <container-id> /bin/sh

# Container stats (CPU, memory, disk)
crictl stats
crictl stats <container-id>

# Stop/remove container
crictl stop <container-id>
crictl rm <container-id>

# Stop/remove pod sandbox
crictl stopp <pod-id>
crictl rmp <pod-id>
```

---

## Failure Mode: Image Pull Issues (Runtime Level)

Unlike Kubernetes `ImagePullBackOff`, runtime-level pull issues happen before the pod is even created.

```bash
# List images cached on node
crictl images
crictl images | grep <image-name>

# Pull image manually to test
crictl pull <registry>/<image>:<tag>

# Pull with auth credentials
crictl pull --auth <base64-encoded-user:pass> <image>

# Remove image
crictl rmi <image-id>

# Remove all unused images (garbage collection)
crictl rmi --prune

# For containerd: use ctr (lower-level)
ctr images list
ctr images pull <image>
ctr images rm <image>

# Check image pull proxy settings
cat /etc/containerd/config.toml | grep -A5 "[plugins.\"io.containerd.grpc.v1.cri\".registry]"

# Containerd registry mirror config
# [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
#   [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
#     endpoint = ["https://my-mirror.example.com"]
```

---

## Failure Mode: Snapshotter / Overlay Filesystem Issues

**Symptom:** container fails to start with `failed to create containerd task`, overlay errors

```bash
# Check snapshotter type
cat /etc/containerd/config.toml | grep snapshotter
# snapshotter = "overlayfs"  (default, requires overlay kernel module)
# snapshotter = "native"     (fallback, no kernel requirements)
# snapshotter = "devmapper"  (for some environments)

# Check if overlay module is loaded
lsmod | grep overlay
modprobe overlay   # load if missing

# Check snapshotter storage
du -sh /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/

# Clean up orphaned snapshots
ctr snapshots ls
ctr snapshots rm <snapshot-key>

# Verify overlay works on the filesystem
mount | grep overlay

# If using XFS (needs ftype=1 for overlay)
xfs_info /dev/sda | grep ftype
# Must show: ftype=1

# Switch to native snapshotter if overlay not working
# In /etc/containerd/config.toml:
# [plugins."io.containerd.grpc.v1.cri".containerd]
#   snapshotter = "native"
```

---

## Failure Mode: OCI Runtime (runc) Errors

**Symptom:** `failed to create shim task: OCI runtime create failed`

```bash
# Check runc version
runc --version

# runc debug
runc --debug state <container-id>

# Common runc errors:
# "operation not permitted" → seccomp or AppArmor blocking syscall
# "no such file" → binary or library missing in container image
# "permission denied" on /proc → /proc mount options issue
# "exec format error" → architecture mismatch (ARM image on x86)

# Check seccomp profile
kubectl get pod <pod> -n <ns> -o jsonpath='{.spec.securityContext.seccompProfile}'
kubectl get pod <pod> -n <ns> -o jsonpath='{.spec.containers[0].securityContext.seccompProfile}'

# Check AppArmor
aa-status | grep -A5 "docker-default\|cri-containerd"

# Container architecture mismatch
kubectl get pod <pod> -n <ns> -o jsonpath='{.spec.nodeSelector}'
# Verify node is correct architecture: kubectl get node <node> -o yaml | grep arch
```

---

## Image Garbage Collection

Kubernetes triggers image GC automatically but you can tune it:

```bash
# kubelet GC configuration (in kubelet config or arguments)
# --image-gc-high-threshold=85   (start GC when disk > 85% full)
# --image-gc-low-threshold=80    (GC until disk < 80% full)
# --minimum-image-ttl-duration=2m (minimum age before eligible for GC)

# Manual trigger (force GC now)
# Not directly possible, but you can:
crictl rmi --prune                 # remove unused images via crictl
ctr images prune --all             # containerd (removes ALL non-running)

# Check disk usage
df -h /var/lib/containerd   # containerd
df -h /var/lib/containers   # CRI-O / podman
```

---

## Quick Comparison: crictl vs kubectl vs docker

| Task | crictl | kubectl | docker |
|------|--------|---------|--------|
| List pods | `crictl pods` | `kubectl get pods` | N/A |
| List containers | `crictl ps` | `kubectl get pods` | `docker ps` |
| Logs | `crictl logs <id>` | `kubectl logs <pod>` | `docker logs <id>` |
| Exec | `crictl exec -it <id>` | `kubectl exec -it <pod>` | `docker exec -it <id>` |
| Images | `crictl images` | N/A | `docker images` |
| Inspect | `crictl inspect <id>` | `kubectl describe pod` | `docker inspect <id>` |
| Pull | `crictl pull <image>` | N/A | `docker pull <image>` |

---

## References

- [crictl user guide](https://kubernetes.io/docs/tasks/debug/debug-cluster/crictl/)
- [containerd docs](https://containerd.io/docs/)
- [CRI-O troubleshooting](https://github.com/cri-o/cri-o/blob/main/docs/crio.8.md)
- [runc](https://github.com/opencontainers/runc)
- [OCI Runtime Spec](https://github.com/opencontainers/runtime-spec)
