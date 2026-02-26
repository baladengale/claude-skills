---
name: k8s-ingress-debug
description: Kubernetes Ingress and LoadBalancer troubleshooting - nginx-ingress, Traefik, cert-manager TLS, external-dns, 404/502/503 errors, SSL certificate issues, and external traffic routing.
metadata:
  emoji: "🚪"
  requires:
    bins: ["kubectl", "bash", "curl"]
---

# K8s Ingress Debug — Ingress & LoadBalancer Troubleshooting Runbook

Systematic diagnostics for Kubernetes Ingress controllers, LoadBalancer Services, TLS certificate management, and external DNS. Covers nginx-ingress, Traefik, cert-manager, and external-dns.

Inspired by: kubernetes/ingress-nginx docs, Traefik docs, cert-manager docs, external-dns docs, Cloudflare Kubernetes Ingress guide.

## When to Activate

Activate when the user asks about:
- Ingress not routing, 404 on ingress, ingress 502/503
- nginx ingress controller issues
- Traefik ingress, Traefik dashboard
- SSL/TLS certificate not working, HTTPS failing
- cert-manager certificate pending, ACME challenge failing
- Let's Encrypt certificate, SSL certificate error
- external-dns not updating, DNS not propagating
- LoadBalancer pending external IP, no external IP
- Ingress class not found, ingressClassName
- Path-based routing, host-based routing not working

## Troubleshooting Runbook

### Step 1 — Ingress Overview

```bash
# All ingress resources
kubectl get ingress -A
kubectl get ingress -n <namespace> -o wide

# Ingress controllers running?
kubectl get pods -n ingress-nginx   # nginx-ingress
kubectl get pods -n traefik         # Traefik

# IngressClass resources
kubectl get ingressclass

# LoadBalancer services
kubectl get svc -A | grep LoadBalancer
```

---

## Failure Mode: Ingress Returns 404

**Root causes:**
1. Ingress has wrong path or host
2. Backend Service selector doesn't match pods
3. Backend Service has no endpoints
4. Ingress class is wrong or missing
5. Path type mismatch (Prefix vs Exact)

**Diagnosis:**

```bash
# Describe ingress — check rules and backend
kubectl describe ingress <name> -n <namespace>

# Check the backend service and endpoints
kubectl get svc <backend-service> -n <namespace>
kubectl get endpoints <backend-service> -n <namespace>
# If endpoints = <none> → service selector mismatch

# Test ingress controller is receiving request
# Find ingress controller pod
IC_POD=$(kubectl get pods -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx -o name | head -1)
kubectl logs $IC_POD -n ingress-nginx --tail=20 | grep -i "error\|404\|backend"

# Check if ingressClassName matches
kubectl get ingress <name> -n <ns> -o jsonpath='{.spec.ingressClassName}'
kubectl get ingressclass

# Test from inside cluster (bypass ingress)
kubectl run curl-test --rm -it --image=curlimages/curl -- \
  curl -v http://<service>.<namespace>.svc.cluster.local:<port>/path

# Test ingress with correct Host header
INGRESS_IP=$(kubectl get ingress <name> -n <ns> -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
curl -H "Host: myapp.example.com" http://$INGRESS_IP/api/v1
```

**Common path type issues:**
```yaml
spec:
  rules:
  - host: myapp.example.com
    http:
      paths:
      - path: /api        # Prefix: matches /api, /api/v1, /api/users
        pathType: Prefix
      - path: /health     # Exact: matches ONLY /health
        pathType: Exact
      - path: /(.*)       # nginx regex (nginx-ingress specific)
        pathType: ImplementationSpecific
```

---

## Failure Mode: Ingress Returns 502/503

**502 Bad Gateway:** nginx can't connect to backend
**503 Service Unavailable:** no healthy backends

```bash
# Check backend pods are Running and Ready
kubectl get pods -n <namespace> -l app=<backend-app>

# Check endpoints are populated
kubectl get endpoints <service> -n <namespace>

# Check if pod is listening on the right port
kubectl exec <pod> -n <ns> -- ss -tulnp | grep <port>
# or
kubectl exec <pod> -n <ns> -- netstat -tlnp | grep <port>

# Check nginx proxy logs for upstream errors
IC_POD=$(kubectl get pods -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx -o name | head -1)
kubectl logs $IC_POD -n ingress-nginx --tail=50 | grep "error\|502\|503\|connect"

# Check nginx ingress annotations for proxy settings
kubectl get ingress <name> -n <ns> -o yaml | grep "nginx.ingress.kubernetes.io"

# Common: proxy-read-timeout too short
kubectl annotate ingress <name> -n <ns> \
  nginx.ingress.kubernetes.io/proxy-read-timeout="600" \
  nginx.ingress.kubernetes.io/proxy-send-timeout="600"
```

---

## Failure Mode: LoadBalancer Pending External IP

**Symptom:** `kubectl get svc` shows `<pending>` in EXTERNAL-IP column

**Root causes:**
1. No cloud provider LoadBalancer support (e.g., bare metal without MetalLB)
2. Cloud provider quota exhausted
3. Wrong subnet/VPC configuration (AWS NLB/ALB)

**Diagnosis:**

```bash
# Check service events
kubectl describe svc <service> -n <namespace>
# Look for: "Error creating load balancer" in Events

# Cloud-provider controller logs
kubectl logs -n kube-system deployment/cloud-controller-manager --tail=30

# AWS: check if service account has LB permissions
# GKE: check if GKE cluster has loadBalancerSourceRanges configured
# AKS: check if node pool has correct subnet

# For bare metal: install MetalLB
kubectl get pods -n metallb-system

# Check MetalLB IP pool
kubectl get ipaddresspools -n metallb-system   # MetalLB v0.13+
kubectl get configmap -n metallb-system config  # MetalLB v0.12-
```

**MetalLB (bare metal LoadBalancer):**
```bash
# Install MetalLB
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.14.5/config/manifests/metallb-native.yaml

# Configure IP pool
kubectl apply -f - <<EOF
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: first-pool
  namespace: metallb-system
spec:
  addresses:
  - 192.168.1.240-192.168.1.250  # range from your LAN
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: example
  namespace: metallb-system
EOF
```

---

## Failure Mode: cert-manager Certificate Pending/Failed

**Understanding cert-manager ACME flow:**
1. Create `Certificate` resource
2. cert-manager creates `CertificateRequest`
3. Creates `Order` → `Challenge` (HTTP-01 or DNS-01)
4. Let's Encrypt validates the challenge
5. Certificate issued and stored as Secret

**Diagnosis:**

```bash
# Check certificate status
kubectl get certificates -n <namespace>
kubectl describe certificate <name> -n <namespace>

# Check CertificateRequest
kubectl get certificaterequest -n <namespace>
kubectl describe certificaterequest <name> -n <namespace>

# Check ACME order
kubectl get orders -n <namespace>
kubectl describe order <name> -n <namespace>

# Check challenge
kubectl get challenges -n <namespace>
kubectl describe challenge <name> -n <namespace>

# cert-manager controller logs
kubectl logs -n cert-manager deployment/cert-manager --tail=30

# Check ClusterIssuer / Issuer
kubectl get clusterissuer
kubectl describe clusterissuer letsencrypt-prod
kubectl get issuer -n <namespace>

# Test ACME HTTP-01 challenge URL directly
# (challenge token must be accessible at this URL)
curl http://<your-domain>/.well-known/acme-challenge/<token>

# Check TLS secret
kubectl get secret <tls-secret-name> -n <namespace> -o yaml
kubectl get secret <tls-secret-name> -n <namespace> \
  -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text -noout
```

**Common cert-manager issues:**

```bash
# HTTP-01 challenge failing: ingress must route /.well-known/acme-challenge/
# Add solver configuration to ClusterIssuer:
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: admin@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx     # must match your ingress class
EOF

# DNS-01 challenge: check DNS provider credentials
kubectl get secret <dns-provider-secret> -n cert-manager

# Rate limited by Let's Encrypt: use staging first
# https://acme-staging-v02.api.letsencrypt.org/directory

# Force certificate renewal
kubectl delete secret <tls-secret-name> -n <namespace>
kubectl delete certificate <name> -n <namespace>
# cert-manager will re-issue
```

---

## nginx-ingress Configuration

```bash
# List all nginx ingress pods (one per node for DaemonSet, or deployment)
kubectl get pods -n ingress-nginx

# Current nginx config
kubectl exec -n ingress-nginx <pod> -- cat /etc/nginx/nginx.conf | grep -A5 "server {"

# nginx-ingress ConfigMap for global settings
kubectl get configmap nginx-configuration -n ingress-nginx -o yaml

# Common annotations:
kubectl annotate ingress <name> -n <ns> \
  nginx.ingress.kubernetes.io/rewrite-target=/ \
  nginx.ingress.kubernetes.io/ssl-redirect="true" \
  nginx.ingress.kubernetes.io/proxy-body-size="50m" \
  nginx.ingress.kubernetes.io/proxy-read-timeout="600"

# Check nginx ingress version and IngressClass
kubectl get ingressclass nginx -o yaml
```

---

## Traefik

```bash
# Traefik pods
kubectl get pods -n traefik

# Traefik IngressRoutes (CRD)
kubectl get ingressroute -A
kubectl describe ingressroute <name> -n <namespace>

# Middleware (auth, rateLimit, etc.)
kubectl get middleware -n <namespace>

# Check Traefik dashboard (port-forward)
kubectl port-forward -n traefik svc/traefik 9000:9000
# Open: http://localhost:9000/dashboard/

# Traefik logs
kubectl logs -n traefik deployment/traefik --tail=30

# Traefik static config
kubectl get configmap traefik -n traefik -o yaml
```

---

## external-dns

```bash
# external-dns pod
kubectl get pods -n external-dns

# external-dns logs (shows what DNS records it's syncing)
kubectl logs -n external-dns deployment/external-dns --tail=30 | \
  grep -E "Create|Update|Delete|error"

# Check annotations on Ingress/Service for external-dns
kubectl get ingress <name> -n <ns> -o yaml | \
  grep "external-dns.alpha.kubernetes.io"

# Add external-dns annotation to ingress
kubectl annotate ingress <name> -n <ns> \
  external-dns.alpha.kubernetes.io/hostname=myapp.example.com \
  external-dns.alpha.kubernetes.io/ttl="60"
```

---

## Quick Connectivity Test

```bash
# From inside cluster — test all ingress rules
kubectl run curl-test --rm -it \
  --image=curlimages/curl \
  -- sh

# Inside pod:
curl -v http://<ingress-ip>/ -H "Host: myapp.example.com"
curl -v https://myapp.example.com/ -k    # -k skips cert validation for testing
curl -v http://myapp.example.com/.well-known/acme-challenge/test  # cert check

# Check ingress from outside
EXTERNAL_IP=$(kubectl get svc ingress-nginx-controller -n ingress-nginx \
  -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
curl -H "Host: myapp.example.com" http://$EXTERNAL_IP/api -v
```

---

## References

- [kubernetes/ingress-nginx troubleshooting](https://kubernetes.github.io/ingress-nginx/troubleshooting/)
- [Traefik Kubernetes Ingress](https://doc.traefik.io/traefik/providers/kubernetes-ingress/)
- [cert-manager Troubleshooting](https://cert-manager.io/docs/troubleshooting/)
- [external-dns](https://github.com/kubernetes-sigs/external-dns)
- [MetalLB](https://metallb.universe.tf/)
