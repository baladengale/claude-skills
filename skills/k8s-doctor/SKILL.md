---
name: k8s-doctor
description: Kubernetes cluster health diagnostics - checks nodes, pods, deployments, and events to surface CrashLoopBackOff, OOMKilled, pending pods, stalled rollouts, and resource pressure with a 0-100 health score. No scripts required — follow this runbook using kubectl via MCP tools.
metadata:
  emoji: "🩺"
  requires:
    bins: ["kubectl"]
---

# K8s Doctor — Kubernetes Cluster Health Diagnostics

A step-by-step diagnostic runbook for assessing Kubernetes cluster health. Follow each section in order, collect the results, and compute a 0–100 health score. No scripts or compiled binaries are needed — every check is a single `kubectl` command you can run via MCP tools.

Inspired by open-source Kubernetes diagnostic patterns from the community (kube-score, kubectl-doctor, robusta).

## When to Activate

Activate when the user asks about:
- Kubernetes cluster health, k8s status, cluster check, k8s doctor
- Pod issues, CrashLoopBackOff, OOMKilled, high restart count
- Node problems, node not ready, node pressure
- Deployment issues, failed rollouts, stalled deployments
- Kubernetes warning events, cluster diagnostics
- Pending pods, failed pods, overall cluster score

---

## How to Use This Runbook

Work through **Steps 1–5** in order. Record every finding as either a **critical issue** (−15 pts each) or a **warning issue** (−5 pts each). At Step 6, subtract from 100 to produce the final health score. Present a summary table and call out every finding with a recommended remediation action.

---

## Step 1 — Identify the Cluster Context

```bash
kubectl config current-context
kubectl cluster-info
kubectl get nodes --show-labels -o wide
```

Note the cluster name, number of nodes, and Kubernetes version. This sets the baseline for the report.

---

## Step 2 — Node Health

```bash
kubectl get nodes -o wide
kubectl describe nodes | grep -E "Conditions:" -A10
```

For each node check the following conditions:

| Condition | Bad value | Severity | Score impact |
|-----------|-----------|----------|--------------|
| Ready | `False` or `Unknown` | **Critical** | −15 |
| MemoryPressure | `True` | Warning | −5 |
| DiskPressure | `True` | Warning | −5 |
| PIDPressure | `True` | Warning | −5 |

**Targeted node detail (when a node is NotReady or under pressure):**

```bash
# Full node conditions with reason/message
kubectl get node <node-name> -o jsonpath='{range .status.conditions[*]}{.type}{"\t"}{.status}{"\t"}{.reason}{"\t"}{.message}{"\n"}{end}'

# Resource allocations on every node
kubectl describe nodes | grep -A8 "Allocated resources:"

# Node-level events
kubectl get events --field-selector involvedObject.kind=Node,type=Warning --sort-by='.lastTimestamp'
```

**Record:** total node count, NotReady nodes (critical), nodes under pressure (warnings).

---

## Step 3 — Pod Health (all namespaces)

```bash
# All pods that are not Running or Completed
kubectl get pods -A --field-selector='status.phase!=Running' | grep -v Completed

# Pods with any restarts — uses jsonpath for reliability across kubectl versions
kubectl get pods -A \
  -o jsonpath='{range .items[?(@.status.containerStatuses[0].restartCount > 0)]}{.metadata.namespace}{"\t"}{.metadata.name}{"\t"}{.status.containerStatuses[0].restartCount}{"\n"}{end}'

# All Warning events related to pods
kubectl get events -A --field-selector=type=Warning --sort-by='.count' | tail -30
```

Classify every unhealthy pod by the table below:

| State / Reason | Severity | Score impact |
|----------------|----------|--------------|
| `CrashLoopBackOff` | **Critical** | −15 |
| `OOMKilled` (current or last state) | **Critical** | −15 |
| `Failed` phase | **Critical** | −15 |
| `ImagePullBackOff` / `ErrImagePull` | Warning | −5 |
| `Pending` (unscheduled) | Warning | −5 |
| Restart count > 10 | Warning | −5 |
| `CreateContainerConfigError` | Warning | −5 |

**Deep-dive on a specific failing pod:**

```bash
# Describe — most informative single command
kubectl describe pod <pod-name> -n <namespace>

# Current logs
kubectl logs <pod-name> -n <namespace>

# Previous container logs (most useful for CrashLoopBackOff)
kubectl logs <pod-name> -n <namespace> --previous

# Container state detail
kubectl get pod <pod-name> -n <namespace> \
  -o jsonpath='{range .status.containerStatuses[*]}{.name}{"\t"}{.state}{"\t"}{.lastState}{"\t"}restarts={.restartCount}{"\n"}{end}'

# Live resource usage (requires metrics-server)
kubectl top pod <pod-name> -n <namespace>
```

**Record:** number of critical pod issues, number of warning pod issues, namespaces affected.

---

## Step 4 — Deployment Health (all namespaces)

```bash
# Deployments with unavailable replicas — uses jsonpath for reliability
kubectl get deployments -A \
  -o jsonpath='{range .items[?(@.status.unavailableReplicas > 0)]}{.metadata.namespace}{"\t"}{.metadata.name}{"\t"}desired={.spec.replicas}{"\t"}ready={.status.readyReplicas}{"\t"}unavailable={.status.unavailableReplicas}{"\n"}{end}'

# Deployments with stalled rollouts
kubectl get deployments -A \
  -o jsonpath='{range .items[*]}{.metadata.namespace}{"\t"}{.metadata.name}{"\t"}{range .status.conditions[?(@.type=="Progressing")]}{.reason}{"\t"}{.message}{"\n"}{end}{end}'
```

Classify deployment issues:

| Condition | Severity | Score impact |
|-----------|----------|--------------|
| `ProgressDeadlineExceeded` (stalled rollout) | **Critical** | −15 |
| Unavailable replicas > 0 | Warning | −5 |

**Deep-dive on a specific deployment:**

```bash
kubectl describe deployment <name> -n <namespace>
kubectl rollout status deployment/<name> -n <namespace>
kubectl rollout history deployment/<name> -n <namespace>
```

**Remediation for stalled rollouts:**
```bash
# Check what changed
kubectl rollout history deployment/<name> -n <namespace> --revision=<N>

# Roll back to last known good revision
kubectl rollout undo deployment/<name> -n <namespace>

# Or roll back to a specific revision
kubectl rollout undo deployment/<name> -n <namespace> --to-revision=<N>
```

**Record:** number of stalled rollouts (critical), deployments with unavailable replicas (warnings).

---

## Step 5 — Warning Events (top 20 by frequency)

```bash
# All Warning events across the cluster, sorted by occurrence count
kubectl get events -A --field-selector=type=Warning \
  -o custom-columns="NAMESPACE:.metadata.namespace,KIND:.involvedObject.kind,NAME:.involvedObject.name,REASON:.reason,COUNT:.count,MESSAGE:.message" \
  --sort-by='.count' | tail -20
```

Events with `COUNT > 5` indicate a recurring problem. Each unique high-frequency event (count > 5) adds one warning issue to the score.

**Common high-frequency events and their meaning:**

| Reason | Likely cause |
|--------|-------------|
| `BackOff` | Container keeps crashing — check logs |
| `FailedScheduling` | Pod cannot be placed — check node capacity / taints |
| `Unhealthy` | Liveness or readiness probe failing |
| `FailedMount` | PVC not bound or volume driver issue |
| `NodeNotReady` | Node lost contact with control plane |
| `Evicted` | Node under disk or memory pressure |
| `OOMKilling` | Process exceeding memory limit on node |

---

## Step 6 — Calculate the Health Score

Use the counts from Steps 2–5:

```
Health Score = 100 − (critical_count × 15) − (warning_count × 5)
Minimum score = 0
```

**Interpreting the score:**

| Score | Status | Recommended action |
|-------|--------|--------------------|
| 90–100 | 🟢 Healthy | No immediate action needed |
| 70–89 | 🟡 Degraded | Investigate warnings, plan remediation |
| 40–69 | 🟠 Unhealthy | Active incidents likely, prioritise fixes |
| 0–39 | 🔴 Critical | Cluster stability at risk, act immediately |

---

## Step 7 — Present the Report

Summarise findings in this format:

```
K8s Doctor — Cluster Health Report
Context  : <cluster-context>
Generated: <timestamp>

Health: <score>/100  Critical: <N>  Warnings: <N>  Nodes: <N>  Pods checked: <N>

=== Node Issues ===
<list each node with its condition>

=== Pod Issues ===
<NAMESPACE  POD-NAME  STATE  RESTARTS  NODE>

=== Deployment Issues ===
<NAMESPACE  DEPLOYMENT  DESIRED  READY  ISSUE>

=== Top Warning Events ===
<NAMESPACE  KIND  NAME  REASON  COUNT  MESSAGE>

=== Recommended Actions ===
<For each critical issue, provide a specific kubectl remediation command>
```

---

## Quick kubectl Reference

```bash
# Cluster context
kubectl config current-context
kubectl config get-contexts

# Node overview
kubectl get nodes -o wide
kubectl top nodes                          # requires metrics-server

# Pod overview
kubectl get pods -A -o wide
kubectl get pods -A --sort-by='.status.containerStatuses[0].restartCount'
kubectl top pods -A --sort-by=memory      # requires metrics-server

# Events
kubectl get events -A --sort-by='.lastTimestamp'
kubectl get events -A --field-selector=type=Warning --sort-by='.count'

# Deployments
kubectl get deployments -A
kubectl rollout status deployment/<name> -n <ns>

# Namespace-scoped checks
kubectl get pods,deployments,events -n <namespace>
```

---

## References

- [Kubernetes: Node Conditions](https://kubernetes.io/docs/concepts/architecture/nodes/#condition)
- [Kubernetes: Pod Lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)
- [Kubernetes: Debugging Pods](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/)
- [Kubernetes: Events](https://kubernetes.io/docs/reference/kubernetes-api/cluster-resources/event-v1/)
- [robusta.dev: Kubernetes Troubleshooting](https://home.robusta.dev/blog/kubernetes-troubleshooting)
