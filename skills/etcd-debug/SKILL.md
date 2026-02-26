---
name: etcd-debug
description: etcd cluster diagnostics - health checks, leader election, defragmentation, backup/restore, slow operations, quorum loss recovery, disk I/O latency, and compaction for Kubernetes control plane stability.
metadata:
  emoji: "🗄️"
  requires:
    bins: ["kubectl", "bash"]
---

# etcd Debug — etcd Cluster Troubleshooting Runbook

etcd is the critical key-value store backing Kubernetes. All cluster state lives here. etcd problems cause API server instability, slow kubectl, stuck controllers, and full cluster outages.

Inspired by: etcd official docs, Kubernetes etcd operations guide, CoreOS/Red Hat etcd runbooks, Rancher etcd recovery docs, OpenShift etcd SRE guides.

## When to Activate

Activate when the user asks about:
- etcd unhealthy, etcd not responding
- API server slow, kubectl timeouts
- etcd leader election, leader change
- etcd defragmentation, disk space full
- etcd backup, snapshot, restore
- etcd quorum lost, cluster split-brain
- etcd slow operations, high latency
- etcd compaction, revision buildup
- etcd TLS certificate issues
- etcd member add/remove, cluster membership

## Troubleshooting Runbook

### Architecture: What is etcd in a Kubernetes Cluster?

```
Kubernetes API Server ──→ etcd cluster (3 or 5 members)
                              ↓
                         Raft consensus (quorum = majority)
                         3-node cluster: needs 2 alive for quorum
                         5-node cluster: needs 3 alive for quorum
```

etcd stores: all Kubernetes objects (pods, deployments, secrets, configmaps, RBAC, CRDs...).

### Step 1 — Accessing etcd (Kubernetes)

etcd is typically a static pod on control-plane nodes. Access it via:

```bash
# Find etcd pods (kubeadm clusters)
kubectl get pods -n kube-system | grep etcd

# Get etcd endpoint and certs from the static pod manifest
cat /etc/kubernetes/manifests/etcd.yaml | grep -E "listen-client-urls|cert-file|key-file|trusted-ca"

# Set etcdctl environment (run ON the control-plane node)
export ETCDCTL_API=3
export ETCDCTL_ENDPOINTS="https://127.0.0.1:2379"
export ETCDCTL_CACERT="/etc/kubernetes/pki/etcd/ca.crt"
export ETCDCTL_CERT="/etc/kubernetes/pki/etcd/server.crt"
export ETCDCTL_KEY="/etc/kubernetes/pki/etcd/server.key"

# OpenShift: etcd pods are in openshift-etcd namespace
kubectl get pods -n openshift-etcd
# Use etcdctl via the etcd container
kubectl exec -n openshift-etcd etcd-<node> -c etcdctl -- etcdctl member list
```

---

## Failure Mode: etcd Health Check Fails

```bash
# Cluster member list (shows endpoint, health, leader)
etcdctl member list -w table

# Health check for all endpoints
etcdctl endpoint health --cluster

# Detailed status (db size, revision, leader)
etcdctl endpoint status --cluster -w table
# Output columns: ENDPOINT | ID | VERSION | DB SIZE | IS LEADER | IS LEARNER | RAFT TERM | RAFT INDEX | ERRORS

# Check for alarm conditions (NOSPACE, CORRUPT)
etcdctl alarm list
```

**Critical thresholds:**
- `DB SIZE` > 2GB → defragment soon; alarm fires at quota (default 2GB → 8GB configurable)
- `RAFT TERM` changes frequently → leader instability
- `ERRORS` column shows `alarm:NOSPACE` → writes blocked, cluster degraded
- Response time > 100ms → I/O bottleneck on etcd node

---

## Failure Mode: etcd Slow / API Server Latency

**Symptoms:** `kubectl` commands take > 5s, API server logs show `etcd request took too long`

**Root cause:** etcd is extremely latency-sensitive. It needs:
- SSD storage (NVMe preferred), never NFS
- Dedicated disk (not shared with kubelet/container logs)
- < 10ms disk fsync latency

**Diagnosis:**

```bash
# Check etcd disk latency (run on etcd node)
# Install: https://github.com/nicowillis/etcd-disk-latency
fio --rw=write --ioengine=sync --fdatasync=1 --directory=/var/lib/etcd \
  --size=22m --bs=2300 --name=etcd-disk-test

# Healthy: 99th percentile < 10ms
# Degraded: > 10ms causes etcd election timeouts

# Check etcd metrics via Prometheus
# Query: histogram_quantile(0.99, rate(etcd_disk_wal_fsync_duration_seconds_bucket[5m]))
# Query: histogram_quantile(0.99, rate(etcd_disk_backend_commit_duration_seconds_bucket[5m]))

# Check etcd slow requests log
kubectl logs -n kube-system etcd-<node> | grep "took too long\|slow"

# Check API server timeout errors
kubectl logs -n kube-system kube-apiserver-<node> | grep "etcd\|timeout\|context deadline"

# CPU/memory on etcd nodes
kubectl top node <control-plane-node>

# Check db size (large db = slow range queries)
etcdctl endpoint status -w table | grep -E "DB SIZE|ENDPOINT"
```

**Remediation:**
```bash
# Move etcd data to faster disk
# Edit /etc/kubernetes/manifests/etcd.yaml
# Change: --data-dir=/var/lib/etcd → /fast-ssd/etcd

# Separate etcd WAL from data (reduces contention)
# --data-dir=/ssd/etcd/data --wal-dir=/nvme/etcd/wal

# Set etcd resource requests/limits (if not already)
# In etcd static pod manifest:
# resources:
#   requests:
#     cpu: 200m
#     memory: 512Mi
```

---

## Failure Mode: etcd DB Size / NOSPACE Alarm

**Symptom:** `alarm:NOSPACE` — etcd is full, writes are blocked, cluster is effectively read-only

**Understanding compaction:**
- etcd keeps all revisions of every key (MVCC)
- Revisions grow over time — compaction removes old revisions
- After compaction → defragmentation frees disk space
- Auto-compaction is usually set (every 5 minutes) but defragmentation must be done manually

```bash
# Check current DB size
etcdctl endpoint status -w table

# Check quota setting
etcdctl alarm list
# alarm:NOSPACE means you hit the quota

# Step 1: Compact to latest revision
REV=$(etcdctl endpoint status --cluster -w json | \
  jq -r '.[0].Status.header.revision')
etcdctl compact $REV

# Step 2: Defragment ALL members (one at a time to maintain quorum)
etcdctl defrag --cluster
# Or per member (safer):
etcdctl defrag --endpoints=https://etcd1:2379
etcdctl defrag --endpoints=https://etcd2:2379
etcdctl defrag --endpoints=https://etcd3:2379

# Step 3: Clear alarm
etcdctl alarm disarm

# Verify size reduced
etcdctl endpoint status -w table

# Increase quota if needed (requires etcd restart)
# In etcd.yaml/manifest: --quota-backend-bytes=8589934592  (8GB)
```

---

## Failure Mode: etcd Leader Lost / Election Issues

```bash
# Check current leader
etcdctl endpoint status --cluster -w table | grep "IS LEADER"

# Check raft term (rapidly increasing = leader instability)
etcdctl endpoint status --cluster -w json | jq -r '.[].Status.raftTerm'

# Monitor leader changes
watch -n2 'etcdctl endpoint status --cluster -w table'

# Check etcd logs for leader changes
kubectl logs -n kube-system etcd-<node> | grep -i "leader\|election\|timeout"

# Common cause: disk I/O too slow → heartbeat timeout → election triggered
# Disk fsync > heartbeat-interval (default 100ms) → election
```

---

## Failure Mode: etcd Member Failure (Quorum at Risk)

**Quorum rule:** need (N/2 + 1) members alive
- 3-node cluster: need 2 alive
- 5-node cluster: need 3 alive

```bash
# Check member health
etcdctl endpoint health --cluster

# Remove failed member and add replacement
# Step 1: Get failed member ID
etcdctl member list

# Step 2: Remove the failed member
etcdctl member remove <member-id>

# Step 3: On the new node, start etcd with --initial-cluster-state=existing
# Step 4: Add new member
etcdctl member add <new-member-name> \
  --peer-urls=https://<new-node-ip>:2380

# Verify new cluster state
etcdctl member list -w table
```

---

## Backup and Restore

**etcd backup = snapshot of entire Kubernetes cluster state**

```bash
# Take snapshot (run on etcd leader node)
ETCDCTL_API=3 etcdctl snapshot save /backup/etcd-$(date +%Y%m%d-%H%M%S).db \
  --endpoints=$ETCDCTL_ENDPOINTS \
  --cacert=$ETCDCTL_CACERT \
  --cert=$ETCDCTL_CERT \
  --key=$ETCDCTL_KEY

# Verify snapshot
etcdctl snapshot status /backup/etcd-snapshot.db -w table

# Restore (DISASTER RECOVERY — stops cluster)
# 1. Stop API server and etcd on ALL control-plane nodes
# 2. On each node:
ETCDCTL_API=3 etcdctl snapshot restore /backup/etcd-snapshot.db \
  --name etcd-node1 \
  --initial-cluster "etcd-node1=https://IP1:2380,etcd-node2=https://IP2:2380,etcd-node3=https://IP3:2380" \
  --initial-cluster-token my-cluster \
  --initial-advertise-peer-urls https://IP1:2380 \
  --data-dir /var/lib/etcd-restored
# 3. Update --data-dir in etcd manifest to point to restored directory
# 4. Restart etcd, then API server
```

---

## etcd on OpenShift

OpenShift manages etcd differently — it's operated by the cluster-etcd-operator:

```bash
# Check etcd operator status
oc get co etcd

# Get etcd pod logs (OpenShift)
oc logs -n openshift-etcd etcd-<node-name> -c etcd

# etcd defragmentation (OpenShift way)
oc rsh -n openshift-etcd etcd-<node>
etcdctl defrag --cluster

# Check etcd backup on OpenShift
# OCP 4.x has built-in backup script
/usr/local/bin/cluster-backup.sh /home/core/backup

# etcd member list on OpenShift
oc rsh -n openshift-etcd etcd-<node> etcdctl member list -w table

# Recover lost master on OpenShift
# https://docs.openshift.com/container-platform/4.x/backup_and_restore/control_plane_backup_and_restore/disaster_recovery/scenario-2-restoring-cluster-state.html
```

---

## etcd Metrics (Prometheus)

```
# Health
up{job="etcd"}

# Disk WAL fsync latency (p99 < 10ms = healthy)
histogram_quantile(0.99, rate(etcd_disk_wal_fsync_duration_seconds_bucket[5m]))

# DB size
etcd_mvcc_db_total_size_in_bytes
etcd_mvcc_db_total_size_in_use_in_bytes

# Leader changes (alert if > 0/5m in production)
increase(etcd_server_leader_changes_seen_total[5m])

# Slow requests
rate(etcd_server_slow_apply_total[5m])
rate(etcd_server_slow_read_indexes_total[5m])

# Client request failures
rate(etcd_server_proposals_failed_total[5m])
```

---

## References

- [etcd Documentation](https://etcd.io/docs/)
- [Kubernetes: Operating etcd](https://kubernetes.io/docs/tasks/administer-cluster/configure-upgrade-etcd/)
- [etcd tuning guide](https://etcd.io/docs/v3.5/tuning/)
- [OpenShift: etcd tasks](https://docs.openshift.com/container-platform/latest/post_installation_configuration/cluster-tasks.html)
- [etcd fio disk benchmark](https://etcd.io/docs/v3.5/op-guide/hardware/#disks)
