---
name: linkerd-debug
description: Linkerd service mesh diagnostics - control plane health checks, proxy injection status, mTLS verification, traffic tap, golden metrics (success rate, latency, RPS), and multicluster debugging.
metadata:
  emoji: "🔗"
  requires:
    bins: ["kubectl", "linkerd", "bash"]
---

# Linkerd Debug — Service Mesh Troubleshooting Runbook

Systematic Linkerd diagnostics using the `linkerd` CLI, `linkerd viz`, and kubectl. Covers control plane health, data plane proxy issues, mTLS, traffic analysis, and multicluster setups.

Inspired by: Linkerd official docs, Buoyant engineering blog, linkerd2-conformance, linkerd-smi.

## When to Activate

Activate when the user asks about:
- Linkerd check failed, Linkerd health check
- Linkerd proxy not injected, sidecar missing
- Linkerd mTLS, certificate rotation
- Linkerd tap, traffic inspection
- Linkerd viz, dashboard, metrics
- Linkerd success rate, golden metrics
- Linkerd ServiceProfile, retries, timeouts
- Linkerd multicluster, service mirroring
- Linkerd policy, authorization policy
- linkerd-jaeger, distributed tracing

## Script Location

```
skills/linkerd-debug/diagnose.sh
```

## Usage

```bash
# Full health check (control + data plane)
bash skills/linkerd-debug/diagnose.sh --check

# Check proxy injection status in namespace
bash skills/linkerd-debug/diagnose.sh -n production --proxies

# Live traffic tap
bash skills/linkerd-debug/diagnose.sh -n production --tap my-deploy

# Golden metrics for namespace
bash skills/linkerd-debug/diagnose.sh -n production --metrics
```

---

## Troubleshooting Runbook

### Step 1 — Control Plane Health

The single most important command for Linkerd:

```bash
# Complete health check (control + data plane)
linkerd check

# Control plane only
linkerd check --pre

# Data plane only
linkerd check --proxy

# Check specific extension
linkerd viz check
linkerd jaeger check
linkerd multicluster check
```

**Interpreting check output:**
- `√` — passing
- `‼` — warning (non-fatal)
- `×` — failing (investigate)

```bash
# Check control plane pods
kubectl get pods -n linkerd
kubectl get pods -n linkerd-viz

# Control plane logs
kubectl logs -n linkerd deployment/linkerd-destination --tail=30
kubectl logs -n linkerd deployment/linkerd-identity --tail=30
kubectl logs -n linkerd deployment/linkerd-proxy-injector --tail=30
```

---

## Failure Mode: Proxy Not Injected

**Symptom:** Pod running without `linkerd-proxy` sidecar container

**Diagnosis:**

```bash
# Check if namespace has injection annotation
kubectl get namespace <ns> -o jsonpath='{.metadata.annotations}'
# Look for: linkerd.io/inject: enabled

# Check pods without proxy
kubectl get pods -n <ns> \
  -o jsonpath='{range .items[*]}{.metadata.name}{" containers: "}{range .spec.containers[*]}{.name}{" "}{end}{"\n"}{end}' | \
  grep -v "linkerd-proxy"

# Check proxy-injector logs
kubectl logs -n linkerd deployment/linkerd-proxy-injector --tail=30

# Check if pod has opt-out annotation
kubectl get pod <pod> -n <ns> -o jsonpath='{.metadata.annotations.linkerd\.io/inject}'
```

**Remediation:**
```bash
# Enable injection on namespace
kubectl annotate namespace <ns> linkerd.io/inject=enabled

# Force reinject by rolling pods
kubectl rollout restart deployment/<name> -n <ns>

# Inject into running deployment (creates new pods)
kubectl get deployment <name> -n <ns> -o yaml | \
  linkerd inject - | kubectl apply -f -

# Opt-out specific pod (add to pod spec annotations)
# linkerd.io/inject: disabled
```

---

## Failure Mode: mTLS Certificate Issues

**Understanding Linkerd mTLS:**
- Linkerd uses automatic mTLS via its own certificate authority
- Identity service (`linkerd-identity`) issues short-lived workload certificates
- Trust anchor (root CA) → issuer certificate → workload certificates

**Diagnosis:**

```bash
# Check certificate expiry
linkerd check --proxy | grep -A5 "identity"

# View certificate details for a pod
linkerd identity -n <ns> <pod>

# Check trust anchor expiry
kubectl get secret linkerd-identity-issuer -n linkerd -o yaml | \
  jq -r '.data."ca.crt"' | base64 -d | openssl x509 -text -noout | \
  grep -A2 "Validity"

# Check identity service
kubectl logs -n linkerd deployment/linkerd-identity --tail=30 | \
  grep -i "error\|fail\|cert"

# Verify mTLS is active between two pods
linkerd tap deployment/<name> -n <ns> | grep -E "tls|mTLS"
```

**Remediation:**
```bash
# Rotate certificates (before expiry)
# Generate new issuer cert or use cert-manager
# See: https://linkerd.io/2/tasks/rotating-control-plane-tls-credentials/

# Check if cert-manager is managing Linkerd certs
kubectl get certificates -n linkerd
kubectl get certificaterequests -n linkerd
```

---

## Traffic Analysis with linkerd tap

`linkerd tap` streams live HTTP/gRPC request data (similar to `tcpdump` but application-layer).

```bash
# Tap all traffic to a deployment
linkerd tap deployment/<name> -n <namespace>

# Tap with method/path filter
linkerd tap deployment/<name> -n <namespace> \
  --method GET \
  --path /api/v1

# Tap from specific source
linkerd tap deployment/<name> -n <namespace> \
  --from deployment/frontend

# Tap with output format
linkerd tap deployment/<name> -n <namespace> -o json | jq .

# Tap a specific pod
linkerd tap pod/<pod-name> -n <namespace>

# Get traffic stats for namespace (golden metrics)
linkerd viz stat -n <namespace> deploy
linkerd viz stat -n <namespace> deploy/<name>

# Top (live traffic table)
linkerd viz top -n <namespace> deploy/<name>

# Edges (mTLS connection map)
linkerd viz edges -n <namespace> deploy
```

---

## Golden Metrics (Success Rate, RPS, Latency)

```bash
# Success rate and RPS for all deployments in namespace
linkerd viz stat deployments -n <namespace>

# Pods
linkerd viz stat pods -n <namespace>

# StatefulSets
linkerd viz stat statefulsets -n <namespace>

# Traffic from one namespace to another
linkerd viz stat deploy -n <target-ns> \
  --from-namespace <source-ns>

# Specific time window
linkerd viz stat deploy -n <namespace> --time-window 10m

# Routes (requires ServiceProfile)
linkerd viz routes deploy/<name> -n <namespace>

# Linkerd viz dashboard
linkerd viz dashboard
# Opens browser at localhost:50750
```

---

## ServiceProfile — Retries and Timeouts

ServiceProfiles define per-route retry/timeout policies and enable route-level metrics.

```bash
# List ServiceProfiles
kubectl get serviceprofiles -n <namespace>
kubectl describe serviceprofile <name>.<namespace>.svc.cluster.local

# Generate ServiceProfile from OpenAPI spec
linkerd profile --open-api swagger.json my-service -n <namespace>

# Generate from live traffic (requires tap data)
linkerd profile --tap deploy/<name> -n <namespace> --tap-duration 30s

# Apply ServiceProfile with retries
kubectl apply -f - <<EOF
apiVersion: linkerd.io/v1alpha2
kind: ServiceProfile
metadata:
  name: my-service.production.svc.cluster.local
  namespace: production
spec:
  routes:
  - name: GET /api/users
    condition:
      method: GET
      pathRegex: /api/users(/.*)?
    responseClasses:
    - condition:
        status:
          min: 500
          max: 599
      isFailure: true
    isRetryable: true
    timeout: 250ms
EOF

# View route metrics after applying ServiceProfile
linkerd viz routes -n <namespace> deploy/<name>
```

---

## Linkerd Multicluster

```bash
# Check multicluster link status
linkerd multicluster check
linkerd multicluster gateways
linkerd multicluster services

# Check service mirror pods
kubectl get pods -n linkerd-multicluster

# View mirrored services
kubectl get services -A | grep "linkerd-remote"

# Debug service mirror controller
kubectl logs -n linkerd-multicluster \
  deployment/linkerd-service-mirror-<cluster-name> --tail=30
```

---

## Linkerd Authorization Policies (L5d 2.11+)

```bash
# List policies
kubectl get authorizationpolicies -n <namespace>
kubectl get meshtlsauthentication -n <namespace>
kubectl get networkauthentication -n <namespace>
kubectl get serverauthorizations -n <namespace>

# Check servers (defines which port/protocol)
kubectl get servers -n <namespace>

# Example: allow only same-namespace traffic
kubectl apply -f - <<EOF
apiVersion: policy.linkerd.io/v1beta1
kind: Server
metadata:
  name: my-server
  namespace: production
spec:
  podSelector:
    matchLabels:
      app: my-app
  port: 8080
  proxyProtocol: HTTP/2
---
apiVersion: policy.linkerd.io/v1beta1
kind: ServerAuthorization
metadata:
  name: my-authz
  namespace: production
spec:
  server:
    name: my-server
  client:
    meshTLS:
      identities:
      - "*.production.serviceaccount.identity.linkerd.cluster.local"
EOF
```

---

## Quick Reference: Linkerd CLI Commands

```bash
linkerd check                    # Full health check
linkerd viz stat deploy -n <ns>  # Golden metrics
linkerd viz top -n <ns>          # Live top
linkerd tap deploy/<name> -n <ns># Live traffic tap
linkerd viz edges -n <ns> deploy # mTLS edge map
linkerd identity -n <ns> <pod>   # Cert info
linkerd inject - | kubectl apply # Inject proxy
linkerd upgrade | kubectl apply  # Upgrade control plane
linkerd viz dashboard            # Open web dashboard
```

---

## References

- [Linkerd Troubleshooting](https://linkerd.io/2/tasks/debugging-your-service/)
- [Linkerd: Distributed Tracing](https://linkerd.io/2/tasks/distributed-tracing/)
- [Linkerd ServiceProfile](https://linkerd.io/2/reference/service-profiles/)
- [Linkerd Authorization Policy](https://linkerd.io/2/reference/authorization-policy/)
- [Linkerd Multicluster](https://linkerd.io/2/tasks/multicluster/)
- [Buoyant Blog — Linkerd Deep Dives](https://buoyant.io/blog/)
