---
name: k8s-doctor
description: Kubernetes cluster health diagnostics - checks nodes, pods, deployments, and events to surface CrashLoopBackOff, OOMKilled, pending pods, stalled rollouts, and resource pressure with a 0-100 health score.
metadata:
  emoji: "🩺"
  requires:
    bins: ["go", "kubectl"]
---

# K8s Doctor — Kubernetes Cluster Health Diagnostics

Kubernetes cluster health tool that shells out to `kubectl`, parses JSON output, and identifies common failure modes. Outputs colored terminal tables with a health score and optional HTML email reports.

Inspired by open-source Kubernetes diagnostic patterns from the community (kube-score, kubectl-doctor, robusta).

## When to Activate

Activate when the user asks about:
- Kubernetes cluster health, k8s status, cluster check
- Pod issues, CrashLoopBackOff, OOMKilled
- Node problems, node not ready, node pressure
- Deployment issues, failed rollouts, stalled deployments
- Kubernetes events, warning events
- K8s doctor, k8s health, cluster diagnostics
- Pending pods, failed pods, high restart count

## Script Location

```
skills/k8s-doctor/main.go
```

## Usage

### Build the binary
```bash
cd skills/k8s-doctor && make build
```

### Run full cluster health check (all namespaces)
```bash
skills/k8s-doctor/k8s-doctor
```

### Check specific namespace
```bash
skills/k8s-doctor/k8s-doctor -n production
```

### Generate HTML report
```bash
skills/k8s-doctor/k8s-doctor -html report.html
```

### Send email report
```bash
skills/k8s-doctor/k8s-doctor -email
```

### Build and run in one step
```bash
cd skills/k8s-doctor && make run
```

## What It Checks

1. **Node health** — Ready status, MemoryPressure, DiskPressure, PIDPressure
2. **Pod health** — CrashLoopBackOff, OOMKilled, ImagePullBackOff, high restarts (>10), Pending, Failed
3. **Deployment health** — Unavailable replicas, ProgressDeadlineExceeded (stalled rollouts)
4. **Warning events** — Recent Kubernetes Warning events sorted by frequency (top 20)

## Health Score

Calculated 0–100:
- Starts at 100
- −15 per critical issue (NotReady node, CrashLoopBackOff, OOMKilled, Failed pod, stalled rollout)
- −5 per warning issue (high restarts, memory/disk pressure, ImagePullBackOff, pending pod)

## Environment Variables

Loaded from `.env` in the current directory (email only):

- `GMAIL_USER` — Gmail address
- `GMAIL_APP_PASSWORD` — Gmail app password
- `K8S_RECIPIENTS` — Comma-separated recipient list

## Dependencies

- `kubectl` configured and pointing to target cluster
- `go` 1.22+ to build from source

## CLI Reference

```
-n string   Namespace to check (default: all namespaces)
-a          Check all namespaces (default: true)
-html file  Save HTML report to file
-email      Send email report
```
