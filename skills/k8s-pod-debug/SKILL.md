---
name: k8s-pod-debug
description: Kubernetes pod failure diagnostics - systematic runbook for CrashLoopBackOff, OOMKilled, ImagePullBackOff, Pending, Evicted, and Init container failures with root-cause analysis and remediation steps.
metadata:
  emoji: "🔬"
  requires:
    bins: ["kubectl", "bash"]
---

# K8s Pod Debug — Pod Failure Runbook

Systematic troubleshooting playbook for Kubernetes pod failures. Covers every common failure mode with exact kubectl commands, root-cause patterns, and remediation steps.

Inspired by: Kubernetes official debugging docs, robusta-dev/robusta, komodor.com, learnk8s.io troubleshooting guides.

## When to Activate

Activate when the user asks about:
- CrashLoopBackOff, crash loop, pod keeps restarting
- OOMKilled, out of memory, pod killed
- ImagePullBackOff, ErrImagePull, image not found
- Pending pod, pod not starting, pod stuck
- Init container failure, init:CrashLoopBackOff
- Pod eviction, evicted pods
- Pod debugging, pod logs, container logs
- Pod not ready, readiness probe failing
- Liveness probe failing, pod health check

## Script Location

```
skills/k8s-pod-debug/diagnose.sh
```

## Usage

```bash
# Quick pod health scan — all namespaces
bash skills/k8s-pod-debug/diagnose.sh

# Check specific namespace
bash skills/k8s-pod-debug/diagnose.sh -n production

# Deep-dive on specific pod
bash skills/k8s-pod-debug/diagnose.sh -p my-pod -n production

# Check all namespaces
bash skills/k8s-pod-debug/diagnose.sh -a
```

---

## Troubleshooting Runbook

### Step 1 — Triage: find unhealthy pods

```bash
# All non-running pods across cluster
kubectl get pods -A --field-selector=status.phase!=Running | grep -v Completed

# Pods with restarts > 5
kubectl get pods -A | awk 'NR>1 && $5>5 {print}'

# Specific namespace
kubectl get pods -n <namespace> -o wide

# Watch pod status changes in real time
kubectl get pods -n <namespace> -w
```

### Step 2 — Describe the pod (always first)

```bash
kubectl describe pod <pod-name> -n <namespace>
```

**Key sections to check in describe output:**
- `Status:` — overall pod phase
- `Conditions:` — PodScheduled, Ready, ContainersReady, Initialized
- `Events:` — most important, shows scheduling failures, pull errors, probe failures
- `Containers > State:` — Waiting/Running/Terminated + reason
- `Containers > Last State:` — previous container state (exit code)
- `Resources:` — requests/limits set

---

## Failure Mode: CrashLoopBackOff

**Symptom:** Pod restarts repeatedly, back-off delay grows (10s → 20s → 40s → 5min cap)

**Root causes:**
1. Application process exits non-zero (bug, missing config, bad env vars)
2. Missing required environment variable or secret
3. Liveness probe too aggressive (kills healthy container)
4. OOMKilled (memory limit too low)
5. Readiness probe misconfigured causing restart cascade

**Diagnosis:**

```bash
# Current logs (may be empty if container just crashed)
kubectl logs <pod> -n <ns>

# Previous container instance logs — MOST USEFUL
kubectl logs <pod> -n <ns> --previous

# Check exit code and termination reason
kubectl get pod <pod> -n <ns> -o jsonpath='{.status.containerStatuses[0].lastState.terminated}'

# Check all env vars injected into pod
kubectl exec <pod> -n <ns> -- env | sort

# Check if secret/configmap referenced exists
kubectl get secret <secret-name> -n <ns>
kubectl get configmap <cm-name> -n <ns>

# Describe to see Events
kubectl describe pod <pod> -n <ns>
```

**Remediation:**
- Exit code 1: Application error → check `--previous` logs for stack trace
- Exit code 137: OOMKilled → increase memory limit or fix memory leak
- Exit code 139: Segfault → application bug, escalate to dev team
- Missing secret: `kubectl create secret generic <name> --from-literal=key=val -n <ns>`
- Liveness probe too tight: increase `initialDelaySeconds` and `failureThreshold`

---

## Failure Mode: OOMKilled

**Symptom:** Container killed by kernel OOM killer, exit code 137

**Diagnosis:**

```bash
# Confirm OOMKilled
kubectl get pod <pod> -n <ns> -o jsonpath='{.status.containerStatuses[*].lastState.terminated.reason}'

# Check current memory limit
kubectl get pod <pod> -n <ns> -o jsonpath='{.spec.containers[*].resources.limits.memory}'

# Check actual memory usage (requires metrics-server)
kubectl top pod <pod> -n <ns>
kubectl top pod -n <ns> --sort-by=memory

# Check node-level OOM events
kubectl describe node <node-name> | grep -A5 "OOM\|memory"

# Check namespace resource quota
kubectl describe resourcequota -n <ns>
```

**Remediation:**
```bash
# Patch deployment to increase memory limit
kubectl patch deployment <name> -n <ns> -p '{"spec":{"template":{"spec":{"containers":[{"name":"<container>","resources":{"limits":{"memory":"512Mi"},"requests":{"memory":"256Mi"}}}]}}}}'

# Or edit directly
kubectl edit deployment <name> -n <ns>
```

**Memory tuning strategy:**
- Set `requests` = steady-state usage (from `kubectl top`)
- Set `limits` = 1.5–2× requests for headroom
- Use VPA (Vertical Pod Autoscaler) for automatic right-sizing

---

## Failure Mode: ImagePullBackOff / ErrImagePull

**Symptom:** Pod cannot pull container image

**Root causes:**
1. Image tag does not exist in registry
2. Private registry — missing or wrong imagePullSecret
3. Registry rate limit (Docker Hub: 100 pulls/6h anonymous)
4. Network policy blocking egress to registry
5. Typo in image name

**Diagnosis:**

```bash
# See exact pull error
kubectl describe pod <pod> -n <ns> | grep -A10 "Events:"

# Check imagePullSecrets on pod
kubectl get pod <pod> -n <ns> -o jsonpath='{.spec.imagePullSecrets}'

# List pull secrets in namespace
kubectl get secrets -n <ns> --field-selector=type=kubernetes.io/dockerconfigjson

# Verify secret content (decode base64)
kubectl get secret <secret> -n <ns> -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d | jq .

# Check if image exists (from a debug pod)
kubectl run debug --rm -it --image=alpine -n <ns> -- \
  wget -qO- https://registry-1.docker.io/v2/<image>/tags/list
```

**Remediation:**
```bash
# Create docker registry secret
kubectl create secret docker-registry regcred \
  --docker-server=<registry> \
  --docker-username=<user> \
  --docker-password=<token> \
  -n <ns>

# Patch service account to use pull secret
kubectl patch serviceaccount default -n <ns> \
  -p '{"imagePullSecrets":[{"name":"regcred"}]}'

# Or add directly to deployment
kubectl set image deployment/<name> <container>=<registry>/<image>:<tag> -n <ns>
```

---

## Failure Mode: Pending Pod

**Symptom:** Pod stays in `Pending` state and never schedules

**Root causes:**
1. Insufficient CPU/memory on any node (scheduler cannot place pod)
2. Node selector / affinity / taints don't match any node
3. PVC not bound (waiting for volume)
4. Pod disruption budget prevents scheduling
5. Namespace resource quota exceeded

**Diagnosis:**

```bash
# Key: check Events in describe — tells you WHY it won't schedule
kubectl describe pod <pod> -n <ns>

# Check node capacity and allocatable
kubectl describe nodes | grep -A8 "Allocatable:"
kubectl get nodes -o custom-columns="NAME:.metadata.name,CPU:.status.allocatable.cpu,MEM:.status.allocatable.memory"

# Check node taints (pod needs matching toleration)
kubectl get nodes -o custom-columns="NAME:.metadata.name,TAINTS:.spec.taints"

# Check pod tolerations
kubectl get pod <pod> -n <ns> -o jsonpath='{.spec.tolerations}'

# Check pod affinity/nodeSelector
kubectl get pod <pod> -n <ns> -o jsonpath='{.spec.nodeSelector}'
kubectl get pod <pod> -n <ns> -o jsonpath='{.spec.affinity}'

# Check namespace quota
kubectl describe resourcequota -n <ns>

# Check PVC status if volume-pending
kubectl get pvc -n <ns>
```

**Remediation:**
- Insufficient resources: scale cluster, add nodes, or reduce pod requests
- Taint mismatch: add toleration to pod spec or remove taint from node
- Quota exceeded: increase quota or clean up unused resources

---

## Failure Mode: Init Container Failure

**Symptom:** `Init:CrashLoopBackOff` or `Init:Error` — main container never starts

**Diagnosis:**

```bash
# List init containers and their status
kubectl get pod <pod> -n <ns> -o jsonpath='{.status.initContainerStatuses[*]}'

# Logs from specific init container
kubectl logs <pod> -n <ns> -c <init-container-name>
kubectl logs <pod> -n <ns> -c <init-container-name> --previous

# Describe to see which init container is failing
kubectl describe pod <pod> -n <ns>
```

**Common init container patterns:**
- DB migration init: waits for database → check DB connectivity
- Config download init: fetches from S3/Vault → check credentials/network
- Dependency wait init: `until nc -z db 5432; do sleep 1; done` → check service DNS

---

## Failure Mode: Readiness Probe Failing

**Symptom:** Pod running but `0/1 READY` — not receiving traffic

**Diagnosis:**

```bash
# Check readiness probe config
kubectl get pod <pod> -n <ns> -o jsonpath='{.spec.containers[0].readinessProbe}'

# Test probe manually from inside pod
kubectl exec <pod> -n <ns> -- wget -qO- http://localhost:8080/healthz
kubectl exec <pod> -n <ns> -- curl -s http://localhost:8080/readyz

# Check Events for probe failure messages
kubectl describe pod <pod> -n <ns> | grep -A5 "Readiness"

# Check endpoint status
kubectl get endpoints <service> -n <ns>
```

**Remediation:**
- Increase `initialDelaySeconds` if app is slow to start
- Adjust `failureThreshold` and `periodSeconds`
- Fix the health check endpoint if it returns errors
- Confirm the probe port matches the container port

---

## Failure Mode: Evicted Pods

**Symptom:** Pod status is `Evicted`

**Root causes:**
1. Node disk pressure (ephemeral storage or imagefs full)
2. Node memory pressure (kubelet evicts lowest-priority pods first)
3. Pod exceeded `ephemeral-storage` limit

**Diagnosis:**

```bash
# Find all evicted pods
kubectl get pods -A --field-selector=status.phase=Failed | grep Evicted

# Get eviction reason
kubectl get pod <pod> -n <ns> -o jsonpath='{.status.message}'

# Check node disk/memory pressure
kubectl describe node <node> | grep -A10 "Conditions:"

# Check node disk usage
kubectl debug node/<node> -it --image=alpine -- df -h

# Check ephemeral storage limits
kubectl get pod <pod> -n <ns> -o jsonpath='{.spec.containers[*].resources.limits.ephemeral-storage}'
```

**Remediation:**
```bash
# Clean up evicted pods
kubectl get pods -A --field-selector=status.phase=Failed \
  -o jsonpath='{range .items[?(@.status.reason=="Evicted")]}{.metadata.namespace} {.metadata.name}{"\n"}{end}' \
  | xargs -n2 sh -c 'kubectl delete pod $2 -n $1' sh

# Increase ephemeral storage limit, set resource requests
# Expand node disk or enable log rotation
```

---

## Quick Reference: kubectl Debug Commands

```bash
# Interactive shell in running pod
kubectl exec -it <pod> -n <ns> -- /bin/sh

# Run ephemeral debug container (K8s 1.23+)
kubectl debug -it <pod> -n <ns> --image=nicolaka/netshoot --target=<container>

# Copy pod with debug tools attached
kubectl debug <pod> -n <ns> --copy-to=<pod>-debug --image=alpine

# Stream logs with timestamps
kubectl logs <pod> -n <ns> --timestamps --follow

# Multi-container pod logs
kubectl logs <pod> -n <ns> -c <container> --previous

# Events sorted by time
kubectl get events -n <ns> --sort-by='.lastTimestamp'
kubectl get events -n <ns> --field-selector=involvedObject.name=<pod>
```

---

## References

- [Kubernetes: Debug Running Pods](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/)
- [Kubernetes: Application Introspection and Debugging](https://kubernetes.io/docs/tasks/debug/debug-application/)
- [learnk8s.io: Troubleshooting Deployments](https://learnk8s.io/troubleshooting-deployments)
- [robusta.dev: Kubernetes Troubleshooting Playbooks](https://home.robusta.dev/blog/kubernetes-troubleshooting)
- [komodor.com: CrashLoopBackOff Guide](https://komodor.com/learn/how-to-fix-crashloopbackoff-kubernetes-error/)
