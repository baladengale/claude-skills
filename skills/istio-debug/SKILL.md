---
name: istio-debug
description: Istio service mesh diagnostics - Envoy proxy config, mTLS verification, traffic routing, circuit breakers, sidecar injection, control plane health, and VirtualService/DestinationRule troubleshooting.
metadata:
  emoji: "🕸️"
  requires:
    bins: ["kubectl", "istioctl", "bash"]
---

# Istio Debug — Service Mesh Troubleshooting Runbook

Systematic Istio/Envoy diagnostics for SRE and platform engineers. Covers control plane health, data plane proxy inspection, mTLS, traffic management, and policy enforcement.

Inspired by: Istio official troubleshooting docs, solo.io guides, tetrate.io blog, istio-by-example.dev.

## When to Activate

Activate when the user asks about:
- Istio troubleshooting, Envoy proxy debug, sidecar issues
- mTLS failing, TLS handshake error, CERTIFICATE_VERIFY_FAILED
- VirtualService not working, traffic not routing correctly
- DestinationRule issues, circuit breaker, outlier detection
- Istio 503, upstream connect error, 503 UC
- Sidecar not injected, istio-proxy not running
- Pilot/Istiod errors, control plane issues
- Istio gateway not routing, ingress gateway 404
- Envoy access logs, proxy-config, proxy-status
- Kiali, Jaeger, Zipkin service mesh observability

## Script Location

```
skills/istio-debug/diagnose.sh
```

## Usage

```bash
# Control plane health check
bash skills/istio-debug/diagnose.sh --control-plane

# Check proxy sync status for all pods
bash skills/istio-debug/diagnose.sh -n production --proxy-status

# Debug specific pod's Envoy config
bash skills/istio-debug/diagnose.sh -n production --pod my-pod --envoy-config

# Verify mTLS between services
bash skills/istio-debug/diagnose.sh -n production --mtls

# Full mesh analysis
bash skills/istio-debug/diagnose.sh --analyze
```

---

## Troubleshooting Runbook

### Step 1 — Control Plane Health

```bash
# Check Istiod (Pilot) health
kubectl get pods -n istio-system
kubectl rollout status deployment/istiod -n istio-system

# Check Istiod logs
kubectl logs -n istio-system -l app=istiod --tail=50

# Istiod memory/CPU
kubectl top pods -n istio-system

# Check Istio version
istioctl version

# Istio configuration validation (catches misconfigs before apply)
istioctl analyze -n <namespace>
istioctl analyze --all-namespaces
```

---

## Failure Mode: Sidecar Not Injected

**Symptom:** Pod running but no `istio-proxy` container; traffic not going through mesh

**Diagnosis:**

```bash
# Check if namespace has injection enabled
kubectl get namespace <ns> --show-labels | grep istio-injection

# Check pod for injection annotation
kubectl get pod <pod> -n <ns> -o jsonpath='{.metadata.annotations}'

# List pods without sidecar in mesh-enabled namespace
kubectl get pods -n <ns> \
  -o jsonpath='{range .items[*]}{.metadata.name}{" "}{range .spec.containers[*]}{.name}{" "}{end}{"\n"}{end}' \
  | grep -v istio-proxy

# Istiod webhook configuration
kubectl get mutatingwebhookconfiguration istio-sidecar-injector -o yaml \
  | grep -E "namespaceSelector|objectSelector"
```

**Remediation:**
```bash
# Enable injection on namespace
kubectl label namespace <ns> istio-injection=enabled

# Force re-inject by rolling pods
kubectl rollout restart deployment/<name> -n <ns>

# Opt-in individual pod (annotation)
# Add to pod spec:
# annotations:
#   sidecar.istio.io/inject: "true"

# Opt-out individual pod
# annotations:
#   sidecar.istio.io/inject: "false"
```

---

## Failure Mode: Proxy Sync / xDS Config Out of Sync

**Symptom:** Traffic routing not matching what's in VirtualService/DestinationRule

**Understanding:** Istiod (Pilot) distributes routing config to all Envoy proxies via xDS API. If proxies are out of sync, traffic rules won't apply.

**Diagnosis:**

```bash
# Check sync status — are all proxies up to date?
istioctl proxy-status
# SYNCED = in sync with Istiod
# STALE = proxy has old config (usually resolves itself)
# NOT SENT = Istiod hasn't sent config yet

# Detailed proxy config for a pod
istioctl proxy-config all <pod> -n <ns>

# Clusters — upstream services known to this proxy
istioctl proxy-config clusters <pod> -n <ns>

# Listeners — ports this proxy listens on
istioctl proxy-config listeners <pod> -n <ns>

# Routes — HTTP routing rules applied
istioctl proxy-config routes <pod> -n <ns>

# Endpoints — upstream pod IPs for each cluster
istioctl proxy-config endpoints <pod> -n <ns>

# Bootstrap — initial Envoy config
istioctl proxy-config bootstrap <pod> -n <ns>
```

---

## Failure Mode: mTLS / TLS Errors

**Symptom:** `CERTIFICATE_VERIFY_FAILED`, `503 UF`, TLS handshake failures in proxy logs

**Understanding mTLS modes:**
- `PERMISSIVE` — accepts both plain text and mTLS
- `STRICT` — requires mTLS (rejects plain text)
- `DISABLE` — no mTLS (plain text only)

**Diagnosis:**

```bash
# Check PeerAuthentication policies
kubectl get peerauthentication -A
kubectl describe peerauthentication -n <ns>

# Check DestinationRule TLS settings
kubectl get destinationrule -A
kubectl get destinationrule <name> -n <ns> -o yaml | grep -A10 "trafficPolicy"

# Verify mTLS is working between two services
istioctl x check-inject -n <ns>

# Check certificate status for a pod's proxy
istioctl proxy-config secret <pod> -n <ns>

# Inspect the certificate details
istioctl proxy-config secret <pod> -n <ns> -o json | \
  jq -r '.dynamicActiveSecrets[0].secret.tlsCertificate.certificateChain.inlineBytes' | \
  base64 -d | openssl x509 -text -noout

# Envoy access log — look for UF (upstream_connection_failure), CERTIFICATE_VERIFY_FAILED
kubectl logs <pod> -n <ns> -c istio-proxy | grep -i "certificate\|TLS\|handshake"
```

**Remediation:**
```bash
# Switch to PERMISSIVE mode to allow plain text (debugging only)
kubectl apply -f - <<EOF
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: default
  namespace: <ns>
spec:
  mtls:
    mode: PERMISSIVE
EOF

# Fix DestinationRule TLS mode mismatch
# If PeerAuthentication is STRICT, DestinationRule must use ISTIO_MUTUAL
kubectl get destinationrule <name> -n <ns> -o yaml
# Ensure: trafficPolicy.tls.mode: ISTIO_MUTUAL
```

---

## Failure Mode: VirtualService / Traffic Routing Not Working

**Symptom:** Traffic not routing to correct version, header-based routing not working, canary not splitting

**Diagnosis:**

```bash
# List all VirtualServices
kubectl get virtualservice -n <ns>
kubectl describe virtualservice <name> -n <ns>

# Check DestinationRules
kubectl get destinationrule -n <ns>
kubectl describe destinationrule <name> -n <ns>

# Validate with istioctl analyze
istioctl analyze -n <ns>

# Check if subset labels match pod labels
kubectl get dr <name> -n <ns> -o jsonpath='{.spec.subsets}'
kubectl get pods -n <ns> --show-labels | grep -E "version|subset"

# Check route is applied in Envoy
istioctl proxy-config routes <pod> -n <ns> --name <port>

# Check cluster with subset
istioctl proxy-config clusters <pod> -n <ns> | grep <service>

# Test routing with curl from a pod
kubectl exec <client-pod> -n <ns> -c istio-proxy -- \
  curl -H "x-my-header: canary" http://<service>:<port>/api
```

**Common VirtualService issues:**
```yaml
# WRONG: host must match service name or FQDN
spec:
  hosts:
  - my-service           # OK for same namespace
  - my-service.ns.svc.cluster.local  # FQDN

# WRONG: weights must sum to 100
http:
- route:
  - destination:
      host: my-service
      subset: v1
    weight: 80
  - destination:
      host: my-service
      subset: v2
    weight: 20   # must total 100

# DestinationRule subset labels must match pod labels exactly
spec:
  subsets:
  - name: v1
    labels:
      version: "1.0"   # pod must have this label
```

---

## Failure Mode: Istio Gateway / Ingress Not Working

**Symptom:** External traffic returns 404, 503, or doesn't reach the service

**Diagnosis:**

```bash
# Check Gateway resources
kubectl get gateway -n <ns>
kubectl get gateway -n istio-system

# Check ingress gateway pod
kubectl get pods -n istio-system -l istio=ingressgateway

# Check gateway service external IP / port
kubectl get svc istio-ingressgateway -n istio-system

# Check gateway logs
kubectl logs -n istio-system -l istio=ingressgateway --tail=50

# Verify VirtualService is bound to Gateway
kubectl get vs <name> -n <ns> -o jsonpath='{.spec.gateways}'

# Check Envoy routes on ingress gateway
istioctl proxy-config routes \
  $(kubectl get pod -n istio-system -l istio=ingressgateway -o jsonpath='{.items[0].metadata.name}') \
  -n istio-system

# Test routing
GATEWAY_IP=$(kubectl get svc istio-ingressgateway -n istio-system \
  -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
curl -H "Host: myapp.example.com" http://$GATEWAY_IP/api
```

---

## Envoy Access Log Interpretation

```bash
# Enable access logging (if not already)
kubectl get configmap istio -n istio-system -o yaml | grep accessLogFile

# Stream proxy access logs
kubectl logs <pod> -n <ns> -c istio-proxy -f

# Key fields in Envoy access log:
# [%START_TIME%] "%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL%"
# %RESPONSE_CODE% %RESPONSE_FLAGS% %BYTES_RECEIVED% %BYTES_SENT%
# %DURATION% %RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)%
# "%REQ(X-FORWARDED-FOR)%" "%REQ(USER-AGENT)%" "%REQ(X-REQUEST-ID)%"
# "%REQ(:AUTHORITY)%" "%UPSTREAM_HOST%"
```

**Response flags (most important):**
| Flag | Meaning | Common Cause |
|------|---------|--------------|
| `UF` | Upstream connection failure | Target pod down, wrong port |
| `UH` | No healthy upstream hosts | All pods unhealthy, DR subset mismatch |
| `URX` | Upstream retry exhausted | Service flapping |
| `NR` | No route match | VirtualService misconfigured |
| `DC` | Downstream connection closed | Client disconnected |
| `UC` | Upstream connection terminated | mTLS mismatch, timeout |
| `RL` | Rate limited | RateLimitFilter or EnvoyFilter applied |
| `-` | No flags | Successful request |

---

## Circuit Breaker / Outlier Detection

```bash
# Check DestinationRule circuit breaker config
kubectl get dr <name> -n <ns> -o yaml | grep -A20 "outlierDetection\|connectionPool"

# Check Envoy outlier detection stats
kubectl exec <pod> -n <ns> -c istio-proxy -- \
  pilot-agent request GET /stats | grep outlier_detection

# Common circuit breaker config
kubectl apply -f - <<EOF
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: my-service-cb
spec:
  host: my-service
  trafficPolicy:
    connectionPool:
      http:
        http1MaxPendingRequests: 100
        http2MaxRequests: 1000
    outlierDetection:
      consecutiveGatewayErrors: 5
      interval: 10s
      baseEjectionTime: 30s
      maxEjectionPercent: 100
EOF
```

---

## Istio Observability Tools

```bash
# Kiali — service mesh topology
kubectl port-forward svc/kiali -n istio-system 20001:20001
# Open: http://localhost:20001

# Jaeger — distributed tracing
kubectl port-forward svc/tracing -n istio-system 80:80
# Open: http://localhost:80

# Prometheus (Istio metrics)
kubectl port-forward svc/prometheus -n istio-system 9090:9090
# Query: istio_requests_total{destination_service="my-svc.ns.svc.cluster.local"}

# Grafana (Istio dashboards)
kubectl port-forward svc/grafana -n istio-system 3000:3000
# Pre-built: Istio Service Dashboard, Istio Workload Dashboard
```

---

## References

- [Istio: Troubleshooting](https://istio.io/latest/docs/ops/diagnostic-tools/)
- [istioctl proxy-config reference](https://istio.io/latest/docs/reference/commands/istioctl/#istioctl-proxy-config)
- [Envoy response flags](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/usage)
- [solo.io: Istio Debugging Guide](https://www.solo.io/blog/debug-istio/)
- [tetrate.io: Istio Troubleshooting](https://tetrate.io/blog/how-to-debug-istio/)
- [Istio by Example](https://istiobyexample.dev/)
