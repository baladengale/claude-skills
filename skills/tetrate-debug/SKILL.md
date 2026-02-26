---
name: tetrate-debug
description: Tetrate Service Bridge (TSB) troubleshooting - tctl CLI, Management Plane/Control Plane/Data Plane health, Workspace and Group configuration, traffic policy, multi-cluster federation, and Istio integration.
metadata:
  emoji: "🌉"
  requires:
    bins: ["kubectl", "tctl", "bash"]
---

# Tetrate Debug — TSB/Tetrate Service Bridge Troubleshooting

Troubleshooting playbook for Tetrate Service Bridge (TSB) — the enterprise multi-cluster service mesh management plane built on top of Istio and Envoy.

Inspired by: Tetrate official docs (docs.tetrate.io), Tetrate Academy training, TSB operator guide, TSB troubleshooting reference.

## When to Activate

Activate when the user asks about:
- Tetrate Service Bridge (TSB), TSB not working
- tctl commands, tctl login, tctl apply
- TSB Management Plane, Control Plane, Data Plane
- TSB Workspace, Group, Tenant config
- TSB traffic policy, TrafficSetting, IngressGateway
- TSB multi-cluster, cluster onboarding
- TSB certificate rotation, TSB trust
- Tetrate service mesh, Tetrate Istio Distribution (TID)
- TSB sync not working, xCP sync issues
- TSB observability, Service Graph, Topology

## Troubleshooting Runbook

### TSB Architecture

```
TSB Management Plane (single cluster)
├── TSB Server (API + UI)
├── MPC (Management Plane Controller)
├── IAM service
├── TSB Postgres database
└── XCP Central (config distribution)

↕ gRPC + TLS (federated across clusters)

TSB Control Plane (each cluster)
├── XCP Edge (sync agent)
├── Istio control plane (Istiod)
└── Tetrate Operator

↕ Envoy xDS

TSB Data Plane (each cluster)
└── Envoy sidecars (via Istio injection)
```

### Step 1 — TSB Login and Connectivity

```bash
# Login to TSB Management Plane
tctl login \
  --username admin \
  --password <password> \
  --org <org-name> \
  --tenant <tenant-name> \
  --server <tsb-address>:8443

# Verify login
tctl get organization
tctl whoami

# Check Management Plane API reachability
curl -k https://<tsb-address>:8443/v2/organizations

# Check TSB server certificate
openssl s_client -connect <tsb-address>:8443 -showcerts 2>/dev/null | \
  openssl x509 -text -noout | grep -E "Subject:|Validity|Not After"
```

---

## Failure Mode: TSB Management Plane Components Down

```bash
# Check Management Plane namespace
kubectl get pods -n tsb

# Key components to check:
# tsb-server (API + UI)
# mpc (management plane controller)
# oap-server (observability — OpenTelemetry)
# xcp-central (config distribution)
# iam-server (identity + access management)

kubectl describe pod -n tsb <failing-pod>
kubectl logs -n tsb deployment/tsb-server --tail=30
kubectl logs -n tsb deployment/mpc --tail=30
kubectl logs -n tsb deployment/xcp-central --tail=30

# TSB database connectivity
kubectl exec -n tsb deployment/tsb-server -- \
  wget -qO- http://localhost:8080/healthz/ready

# TSB Postgres
kubectl get pods -n tsb | grep postgres
kubectl logs -n tsb deployment/tsb-postgres --tail=20
```

---

## Failure Mode: Control Plane Sync (XCP Edge) Not Working

**XCP Edge** is the sync agent in each workload cluster. It receives config from XCP Central (management plane) and pushes it to local Istiod/Envoy.

```bash
# Check XCP Edge in workload cluster
kubectl get pods -n istio-system | grep xcp

# XCP Edge logs (shows sync status)
kubectl logs -n istio-system deployment/xcp-edge --tail=30
kubectl logs -n istio-system deployment/xcp-edge --tail=50 | \
  grep -iE "error|fail|sync|connected|disconnected"

# Check XCP Edge config
kubectl get configmap xcp-edge -n istio-system -o yaml

# XCP Edge should show connection to management plane:
# "Connected to XCP Central at <tsb-address>:9443"

# Check certificates
kubectl get secret xcp-edge-cert -n istio-system -o yaml | \
  jq -r '.data["tls.crt"]' | base64 -d | openssl x509 -text -noout | \
  grep "Validity" -A3

# Check network connectivity from cluster to management plane
kubectl run netshoot --rm -it -n istio-system \
  --image=nicolaka/netshoot -- \
  nc -zv <tsb-mp-address> 9443
```

---

## Failure Mode: Cluster Not Onboarded to TSB

```bash
# List clusters in TSB
tctl get cluster

# Check cluster status
tctl get cluster <cluster-name> -o yaml

# Re-generate cluster tokens if expired
tctl install cluster-operators \
  --cluster <cluster-name> \
  --registry <registry>

# Apply cluster onboarding manifest
tctl install manifest cluster-operators \
  --cluster <cluster-name> > cluster-operators.yaml
kubectl apply -f cluster-operators.yaml

# Check Tetrate Operator in workload cluster
kubectl get pods -n operators | grep tetrate
kubectl logs -n operators deployment/tsb-operator --tail=30
```

---

## Failure Mode: Workspace / Group Config Not Applying

**TSB hierarchy:** Organization → Tenant → Workspace → Group → Policy

```bash
# List workspaces
tctl get workspace -t <tenant>

# Get workspace config
tctl get workspace <ws-name> -t <tenant> -o yaml

# List groups in workspace
tctl get trafficgroup -w <workspace> -t <tenant>
tctl get securitygroup -w <workspace> -t <tenant>
tctl get gatewaygroup -w <workspace> -t <tenant>

# Check config push status
tctl get trafficgroup <group> -w <workspace> -t <tenant> -o yaml | \
  grep -A10 "status:"

# Common: namespace not included in workspace
# Workspace must have namespace selectors that match the target namespace
tctl get workspace <ws> -t <tenant> -o yaml | grep -A10 "namespaceSelector"

# Apply workspace config
tctl apply -f workspace.yaml

# Validate TSB config before applying
tctl x config validate -f workspace.yaml
```

---

## TSB Traffic Policy

```bash
# TrafficSettings — mTLS, retry, timeout
tctl get trafficsettings -w <workspace> -g <group> -t <tenant>

# IngressGateway — expose services via TSB-managed gateway
tctl get ingressgateway -w <workspace> -g <group> -t <tenant>

# EgressGateway
tctl get egressgateway -w <workspace> -g <group> -t <tenant>

# Check VirtualService generated by TSB (in Kubernetes)
kubectl get virtualservice -n <namespace>
kubectl describe virtualservice <name> -n <namespace>

# Check DestinationRule generated by TSB
kubectl get destinationrule -n <namespace>

# TSB overrides Istio resources directly — don't manually edit
# VirtualService/DestinationRule managed by TSB will be overwritten
```

---

## TSB Certificate Management

```bash
# Check TSB certificates
tctl get certificate -o yaml

# List certificate bundles
kubectl get secret -n tsb | grep cert
kubectl get secret -n istio-system | grep cert

# TSB uses cert-manager or its own PKI
kubectl get pods -n cert-manager
kubectl get certificates -n tsb
kubectl get certificates -n istio-system

# Rotate XCP Edge cert
# Delete old cert secret → TSB operator regenerates
kubectl delete secret xcp-edge-cert -n istio-system
# Wait for operator to recreate

# Check trust bundle (root CA)
kubectl get configmap istio-ca-root-cert -n istio-system -o yaml
```

---

## TSB Observability (Service Graph)

```bash
# TSB uses OAP (OpenTelemetry Collector + SkyWalking backend)
kubectl get pods -n tsb | grep oap
kubectl logs -n tsb deployment/oap-server --tail=20

# Check if Envoy is sending traces
kubectl exec <pod> -n <ns> -c istio-proxy -- \
  pilot-agent request GET /stats | grep "tracing"

# TSB UI: Service Graph
# Access via TSB console: https://<tsb-address>:8443/
# Service Graph shows RED metrics per service

# Envoy OTel export check
kubectl exec <pod> -n <ns> -c istio-proxy -- \
  pilot-agent request GET /stats | grep "opentelemetry_exporter"
```

---

## tctl Quick Reference

```bash
# Auth
tctl login --server <addr>:8443 --username admin --org <org>
tctl whoami

# Resources (always specify -t <tenant> -w <workspace> -g <group>)
tctl get organization
tctl get tenant -o <org>
tctl get workspace -t <tenant>
tctl get trafficgroup -w <ws> -t <tenant>
tctl get cluster

# Apply / delete
tctl apply -f config.yaml
tctl delete workspace <name> -t <tenant>

# Validate
tctl x config validate -f config.yaml

# Generate install manifests
tctl install manifest management-plane    # management plane
tctl install manifest control-plane       # workload cluster control plane
tctl install manifest cluster-operators   # onboard a cluster

# Describe (like kubectl describe)
tctl describe workspace <name> -t <tenant>

# Get in different formats
tctl get workspace <name> -t <tenant> -o yaml
tctl get workspace <name> -t <tenant> -o json
```

---

## References

- [Tetrate Service Bridge Docs](https://docs.tetrate.io/service-bridge/)
- [TSB Troubleshooting Guide](https://docs.tetrate.io/service-bridge/latest/operations/troubleshooting)
- [Tetrate Academy (free training)](https://academy.tetrate.io/)
- [TSB Architecture](https://docs.tetrate.io/service-bridge/latest/concepts/architecture)
- [Tetrate Istio Distribution (TID)](https://tetrate.io/tetrate-istio-distro/)
