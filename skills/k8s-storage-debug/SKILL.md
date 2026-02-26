---
name: k8s-storage-debug
description: Kubernetes storage troubleshooting - PersistentVolume and PVC binding failures, StorageClass issues, volume mount errors, ReadWriteMany access modes, StatefulSet storage, and CSI driver diagnostics.
metadata:
  emoji: "💾"
  requires:
    bins: ["kubectl", "bash"]
---

# K8s Storage Debug — Storage Troubleshooting Runbook

Systematic storage diagnostics for Kubernetes persistent volumes, claims, StorageClasses, and CSI drivers. Covers all common failure modes from PVC stuck in Pending to volume mount errors.

Inspired by: Kubernetes storage SIG docs, Rook/Ceph debugging guides, AWS EBS/EFS CSI docs, Longhorn troubleshooting guide.

## When to Activate

Activate when the user asks about:
- PVC pending, PersistentVolumeClaim not bound
- PV not available, volume not binding
- Volume mount error, failed to mount
- ReadWriteMany access mode issues
- StorageClass not found, provisioner error
- CSI driver error, CSI node not running
- StatefulSet volume, volumeClaimTemplate issue
- Rook, Ceph, Longhorn storage issues
- NFS mount failing, AWS EBS not attaching
- Volume expansion, resize PVC

## Troubleshooting Runbook

### Step 1 — Storage Status Overview

```bash
# All PVs (cluster-scoped)
kubectl get pv

# All PVCs across namespaces
kubectl get pvc -A

# PVCs not in Bound state
kubectl get pvc -A | grep -v Bound

# StorageClasses
kubectl get storageclass
kubectl get storageclass -o custom-columns="NAME:.metadata.name,PROVISIONER:.provisioner,RECLAIM:.reclaimPolicy,VOLUMEBINDING:.volumeBindingMode"

# CSI drivers
kubectl get csidrivers
kubectl get csinodes
```

---

## Failure Mode: PVC Stuck in Pending

**Symptom:** PVC shows `Pending` status — pod using it also stuck in `Pending`

**Root causes:**
1. No PV available that matches the PVC (capacity, access mode, StorageClass)
2. StorageClass provisioner not running or misconfigured
3. No `storageClassName` and no default StorageClass
4. Volume binding mode is `WaitForFirstConsumer` (normal — binds when pod schedules)
5. Cloud provider quota exhausted (e.g., EBS volume limit)

**Diagnosis:**

```bash
# Describe the PVC — Events section is key
kubectl describe pvc <pvc-name> -n <namespace>

# Check if StorageClass exists and has the right provisioner
kubectl get sc <storage-class-name>
kubectl describe sc <storage-class-name>

# Check provisioner pod is running
kubectl get pods -A | grep -E "provisioner|csi"

# Check if there are matching PVs (static provisioning)
kubectl get pv -o custom-columns="NAME:.metadata.name,CAPACITY:.spec.capacity.storage,ACCESS:.spec.accessModes,STATUS:.status.phase,CLAIM:.spec.claimRef.name,SC:.spec.storageClassName"

# Check events across namespace
kubectl get events -n <namespace> --sort-by='.lastTimestamp' | grep -i "pvc\|volume\|provision"

# Check CSI node plugin is running on ALL nodes
kubectl get pods -n kube-system | grep csi

# AWS EBS CSI specific
kubectl logs -n kube-system -l app=ebs-csi-controller --tail=30
kubectl logs -n kube-system -l app=ebs-csi-node --tail=30
```

**Remediation:**

```bash
# Fix 1: Create a default StorageClass
kubectl patch storageclass <sc-name> \
  -p '{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'

# Fix 2: Create a PV manually (static provisioning)
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolume
metadata:
  name: my-pv
spec:
  capacity:
    storage: 10Gi
  accessModes:
  - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: manual
  hostPath:
    path: /mnt/data
EOF

# Fix 3: Check WaitForFirstConsumer mode — PVC binds when pod is scheduled
kubectl get sc <sc-name> -o jsonpath='{.volumeBindingMode}'
# If "WaitForFirstConsumer", PVC will bind once the consuming pod is scheduled — this is NORMAL
```

---

## Failure Mode: Volume Mount Error

**Symptom:** Pod stuck in `ContainerCreating`, error: `failed to mount volume`

**Diagnosis:**

```bash
# Describe pod for mount error details
kubectl describe pod <pod> -n <namespace>
# Look for: "MountVolume.SetUp failed" or "Unable to attach" in Events

# Check PV is bound to this PVC
kubectl get pvc <pvc-name> -n <namespace>
kubectl describe pv $(kubectl get pvc <pvc-name> -n <namespace> -o jsonpath='{.spec.volumeName}')

# Check CSI node logs on the node where pod is scheduled
POD_NODE=$(kubectl get pod <pod> -n <ns> -o jsonpath='{.spec.nodeName}')
CSI_NODE_POD=$(kubectl get pods -n kube-system -l app=<csi-node-label> \
  --field-selector="spec.nodeName=$POD_NODE" -o name)
kubectl logs $CSI_NODE_POD -n kube-system -c <csi-driver-container> --tail=30

# Common: stale NFS mount / volume attached to wrong node
kubectl get volumeattachments

# Detach hung volume (AWS EBS example)
aws ec2 detach-volume --volume-id <vol-id> --force
```

---

## Failure Mode: Access Mode Mismatch

**Understanding access modes:**

| Access Mode | Short | Meaning |
|------------|-------|---------|
| `ReadWriteOnce` | RWO | One node read/write (most block storage) |
| `ReadOnlyMany` | ROX | Multiple nodes read-only |
| `ReadWriteMany` | RWX | Multiple nodes read/write (NFS, CephFS, EFS) |
| `ReadWriteOncePod` | RWOP | Single pod only (K8s 1.22+) |

```bash
# Check PVC access mode request
kubectl get pvc <pvc-name> -n <ns> -o jsonpath='{.spec.accessModes}'

# Check PV access modes
kubectl get pv <pv-name> -o jsonpath='{.spec.accessModes}'

# Use NFS or CephFS for ReadWriteMany
# AWS: use EFS CSI driver for RWX
# GKE: use Filestore CSI driver for RWX
```

---

## Failure Mode: Volume Expansion (Resize) Failing

**Symptom:** PVC stuck after editing storage size

```bash
# Check if StorageClass allows expansion
kubectl get sc <sc-name> -o jsonpath='{.allowVolumeExpansion}'
# Must be: true

# Check PVC expansion status
kubectl describe pvc <pvc-name> -n <namespace>
# Look for: "Resizing" condition

# If expansion is stuck — restart the pod using the PVC
kubectl delete pod <pod-using-pvc> -n <namespace>
# After pod restart, file system resize happens automatically

# Check resize condition
kubectl get pvc <pvc-name> -n <namespace> \
  -o jsonpath='{.status.conditions}'
```

---

## StatefulSet Storage

StatefulSets use `volumeClaimTemplates` — each pod gets its own PVC.

```bash
# Check PVCs created by StatefulSet
kubectl get pvc -n <namespace> -l app=<statefulset-name>

# PVC naming pattern: <volumeClaimTemplate.name>-<statefulset-name>-<ordinal>
# Example: data-mysql-0, data-mysql-1, data-mysql-2

# If a StatefulSet pod is stuck — check its specific PVC
kubectl describe pvc data-<statefulset>-<ordinal> -n <namespace>

# Delete and recreate specific StatefulSet pod (reattaches existing PVC)
kubectl delete pod <statefulset-name>-<ordinal> -n <namespace>

# WARNING: Deleting a StatefulSet does NOT delete its PVCs (by design)
# To delete PVCs manually:
kubectl delete pvc -l app=<statefulset-name> -n <namespace>
```

---

## Rook/Ceph Storage

```bash
# Ceph cluster health
kubectl exec -n rook-ceph \
  $(kubectl get pod -n rook-ceph -l app=rook-ceph-tools -o name) \
  -- ceph status

# Check OSD status
kubectl exec -n rook-ceph \
  $(kubectl get pod -n rook-ceph -l app=rook-ceph-tools -o name) \
  -- ceph osd status

# Pool usage
kubectl exec -n rook-ceph \
  $(kubectl get pod -n rook-ceph -l app=rook-ceph-tools -o name) \
  -- ceph df

# Fix PG inconsistency
kubectl exec -n rook-ceph \
  $(kubectl get pod -n rook-ceph -l app=rook-ceph-tools -o name) \
  -- ceph health detail
```

---

## Longhorn Storage

```bash
# Longhorn system status
kubectl get pods -n longhorn-system

# Check volume health
kubectl get volumes -n longhorn-system
kubectl get replicas -n longhorn-system

# Longhorn UI
kubectl port-forward -n longhorn-system svc/longhorn-frontend 8080:80
# Open: http://localhost:8080

# Rebuild degraded volume
kubectl get volumes -n longhorn-system | grep -v Healthy
kubectl patch volume <vol-name> -n longhorn-system \
  --type merge -p '{"spec":{"numberOfReplicas":3}}'
```

---

## Volume Troubleshooting Commands Reference

```bash
# Verify filesystem on volume (requires exec into pod)
kubectl exec <pod> -n <ns> -- df -h /mount/path
kubectl exec <pod> -n <ns> -- ls -la /mount/path

# Check mount permissions
kubectl exec <pod> -n <ns> -- stat /mount/path

# Check if volume is read-only (causes write errors)
kubectl exec <pod> -n <ns> -- touch /mount/path/test && echo "writable" || echo "read-only"

# List all VolumeAttachments (shows which volume is attached to which node)
kubectl get volumeattachments

# Delete stale VolumeAttachment (if volume stuck attaching)
kubectl delete volumeattachment <name>

# Check storage capacity on nodes (K8s 1.21+ CSIStorageCapacity)
kubectl get csistoragecapacities -A
```

---

## References

- [Kubernetes: Persistent Volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/)
- [Kubernetes: Storage Classes](https://kubernetes.io/docs/concepts/storage/storage-classes/)
- [AWS EBS CSI Driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver)
- [Rook/Ceph Troubleshooting](https://rook.io/docs/rook/latest/Troubleshooting/ceph-common-issues/)
- [Longhorn Troubleshooting](https://longhorn.io/docs/latest/troubleshooting/)
- [CSI Spec](https://github.com/container-storage-interface/spec)
