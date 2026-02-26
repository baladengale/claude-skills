---
name: k8s-hpa-debug
description: Kubernetes autoscaling troubleshooting - HPA not scaling, metrics-server missing, KEDA ScaledObjects, VPA recommendations, custom/external metrics, and resource request tuning for effective autoscaling.
metadata:
  emoji: "📈"
  requires:
    bins: ["kubectl", "bash"]
---

# K8s HPA Debug — Autoscaling Troubleshooting Runbook

Diagnostics for Kubernetes Horizontal Pod Autoscaler (HPA), Vertical Pod Autoscaler (VPA), and KEDA (Kubernetes Event-Driven Autoscaling). Covers scaling failures, metrics pipeline issues, and configuration tuning.

Inspired by: Kubernetes HPA docs, KEDA project, VPA docs, kube-metrics-adapter, Prometheus Adapter guide.

## When to Activate

Activate when the user asks about:
- HPA not scaling, pods not scaling up/down
- Metrics server not available, unknown metrics
- HPA shows <unknown>/50% for CPU
- KEDA ScaledObject not triggering
- VPA recommendations, vertical scaling
- Custom metrics HPA, external metrics
- Prometheus Adapter, metrics-server
- Scale to zero, scale from zero
- HPA flapping, rapid scale up/down
- Resource requests too low/high

## Troubleshooting Runbook

### Step 1 — HPA Status Overview

```bash
# List all HPAs (shows current/desired replicas and metrics)
kubectl get hpa -n <namespace>
kubectl get hpa -A

# Watch HPA scaling in real time
kubectl get hpa -n <namespace> -w

# Detailed HPA status
kubectl describe hpa <hpa-name> -n <namespace>
# Key sections: Metrics (current vs target), Events, Conditions
```

---

## Failure Mode: HPA Shows `<unknown>` Metrics

**Symptom:** `kubectl get hpa` shows `<unknown>/50%` for CPU or memory

**Root cause:** metrics-server is not installed or not working

**Diagnosis:**

```bash
# Check if metrics-server is installed
kubectl get pods -n kube-system | grep metrics-server

# Test metrics-server API
kubectl top nodes
kubectl top pods -n <namespace>
# If these fail → metrics-server is broken

# Check metrics-server logs
kubectl logs -n kube-system deployment/metrics-server --tail=30

# Check metrics-server args (common issue: missing --kubelet-insecure-tls)
kubectl get deployment metrics-server -n kube-system -o yaml | \
  grep -A10 "args:"

# Check metrics API availability
kubectl api-resources | grep "metrics.k8s.io"
kubectl get --raw "/apis/metrics.k8s.io/v1beta1/nodes" 2>&1
```

**Remediation:**
```bash
# Install metrics-server (with kubelet-insecure-tls for self-signed certs)
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# Patch for clusters with self-signed certificates
kubectl patch deployment metrics-server -n kube-system \
  --type=json \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'

# For minikube / kind
minikube addons enable metrics-server
# or
helm install metrics-server metrics-server/metrics-server -n kube-system \
  --set args[0]="--kubelet-insecure-tls"
```

---

## Failure Mode: HPA Not Scaling Up

**Symptom:** Load is high but HPA doesn't increase replicas

**Diagnosis:**

```bash
# Check HPA events and conditions
kubectl describe hpa <hpa-name> -n <namespace>

# Key conditions to check:
# AbleToScale: True/False
# ScalingActive: True/False (False = metrics not found)
# ScalingLimited: True/False (True = at maxReplicas)

# Check if pods have resource REQUESTS set (required for CPU HPA)
kubectl get pods -n <namespace> \
  -o custom-columns="NAME:.metadata.name,CPU_REQ:.spec.containers[*].resources.requests.cpu"

# If requests are <none> → HPA cannot calculate CPU %
# CPU % = actual usage / request

# Check current metrics being seen by HPA
kubectl get --raw "/apis/metrics.k8s.io/v1beta1/namespaces/<ns>/pods" | jq .

# Check if at maxReplicas (scaling is correctly limited)
kubectl get hpa <name> -n <ns> -o jsonpath='{.spec.maxReplicas}'

# Check scaleTargetRef is correct
kubectl get hpa <name> -n <ns> -o jsonpath='{.spec.scaleTargetRef}'
```

**Fix: missing resource requests:**
```bash
kubectl patch deployment <name> -n <ns> \
  -p '{"spec":{"template":{"spec":{"containers":[{"name":"<container>","resources":{"requests":{"cpu":"100m","memory":"128Mi"}}}]}}}}'
```

---

## Failure Mode: HPA Not Scaling Down / Flapping

**Symptom:** HPA scales up aggressively but won't scale down, or scales up/down repeatedly

**Understanding HPA behavior:**
- Scale-down cooldown: default 5 minutes (stabilizationWindowSeconds)
- Scale-down: conservative by default (takes max of last 5 min readings)
- Scale-up: immediate (takes max of last 3 min readings by default)

**Diagnosis:**

```bash
# Check stabilization window settings
kubectl get hpa <name> -n <ns> -o yaml | grep -A10 "behavior:"

# Check current replica count vs target
kubectl describe hpa <name> -n <ns> | grep -E "Replicas:|Current Replicas|Desired Replicas"

# Check if deployments have PodDisruptionBudget blocking scale-down
kubectl get pdb -n <ns>
kubectl describe pdb -n <ns>
```

**Tune scale-down behavior:**
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: my-app
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app
  minReplicas: 2
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300  # 5 min cooldown (default)
      policies:
      - type: Pods
        value: 2
        periodSeconds: 60              # scale down max 2 pods/min
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
      - type: Percent
        value: 100
        periodSeconds: 15              # double replicas every 15s
```

---

## Custom Metrics HPA (Prometheus Adapter)

```bash
# Check if custom metrics API is available
kubectl api-resources | grep custom.metrics
kubectl get --raw "/apis/custom.metrics.k8s.io/v1beta1" | jq .

# Check Prometheus Adapter
kubectl get pods -n monitoring | grep prometheus-adapter
kubectl logs -n monitoring deployment/prometheus-adapter --tail=30

# List available custom metrics
kubectl get --raw "/apis/custom.metrics.k8s.io/v1beta1" | jq '.resources[].name'

# External metrics API
kubectl get --raw "/apis/external.metrics.k8s.io/v1beta1" | jq .

# Example HPA with custom metric (requests per second)
kubectl apply -f - <<EOF
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
spec:
  metrics:
  - type: Pods
    pods:
      metric:
        name: http_requests_per_second
      target:
        type: AverageValue
        averageValue: 100   # scale when > 100 RPS per pod
EOF
```

---

## KEDA — Event-Driven Autoscaling

KEDA scales based on external event sources: Kafka lag, queue length, Prometheus queries, cron, etc.

```bash
# Check KEDA installation
kubectl get pods -n keda

# List ScaledObjects
kubectl get scaledobject -A
kubectl describe scaledobject <name> -n <namespace>

# List ScaledJobs (for batch workloads)
kubectl get scaledjob -A

# KEDA operator logs
kubectl logs -n keda deployment/keda-operator --tail=30

# Check ScaledObject conditions
kubectl get scaledobject <name> -n <ns> -o yaml | grep -A10 "conditions:"

# Common KEDA trigger examples:
# - Kafka consumer group lag
# - RabbitMQ queue depth
# - Azure Service Bus queue length
# - Prometheus query result
# - Cron schedule

# Check trigger authentication
kubectl get triggerauthentication -n <namespace>
kubectl get clustertriggerauthentication

# Debug: check scaled object status
kubectl get scaledobject <name> -n <ns> \
  -o jsonpath='{.status.conditions[*]}'
```

**Example KEDA ScaledObject (Kafka):**
```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: kafka-scaler
  namespace: production
spec:
  scaleTargetRef:
    name: my-consumer
  minReplicaCount: 0    # scale to zero!
  maxReplicaCount: 30
  cooldownPeriod: 300
  triggers:
  - type: kafka
    metadata:
      bootstrapServers: kafka:9092
      consumerGroup: my-group
      topic: my-topic
      lagThreshold: "100"   # scale when lag > 100 messages/replica
```

---

## VPA — Vertical Pod Autoscaler

VPA automatically sets CPU/memory requests based on actual usage.

```bash
# Check VPA installation
kubectl get pods -n kube-system | grep vpa
kubectl api-resources | grep verticalpodautoscaler

# List VPA objects
kubectl get vpa -n <namespace>

# Check VPA recommendations
kubectl describe vpa <name> -n <namespace>
# Look for: Recommendation section with Lower/Target/Upper bounds

# Get recommendations in JSON
kubectl get vpa <name> -n <namespace> \
  -o jsonpath='{.status.recommendation}'

# VPA modes:
# Off: recommendation only, no auto-apply
# Initial: set requests only at pod creation
# Auto: evict and recreate pods with new requests (default)
# Recreate: like Auto but allows immediate eviction

# Apply VPA in "Off" mode (recommendation only - safe for production)
kubectl apply -f - <<EOF
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: my-app-vpa
  namespace: production
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app
  updatePolicy:
    updateMode: "Off"   # recommendation only
  resourcePolicy:
    containerPolicies:
    - containerName: "*"
      minAllowed:
        cpu: 50m
        memory: 64Mi
      maxAllowed:
        cpu: 4
        memory: 4Gi
EOF
```

**NOTE:** Do not use HPA (CPU/memory) and VPA (Auto) together on same deployment — they conflict. Use VPA for requests, HPA for replica count with custom/external metrics.

---

## References

- [Kubernetes HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)
- [HPA Walkthrough](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale-walkthrough/)
- [metrics-server](https://github.com/kubernetes-sigs/metrics-server)
- [KEDA](https://keda.sh/docs/latest/troubleshooting/)
- [VPA](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler)
- [Prometheus Adapter](https://github.com/kubernetes-sigs/prometheus-adapter)
