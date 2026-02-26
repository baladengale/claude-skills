---
name: k8s-network-debug
description: Kubernetes networking diagnostics - DNS resolution failures, Service connectivity, NetworkPolicy blocking, CNI issues, and pod-to-pod communication debugging with netshoot and kubectl.
metadata:
  emoji: "🌐"
  requires:
    bins: ["kubectl", "bash"]
---

# K8s Network Debug — Networking Troubleshooting Runbook

Comprehensive networking diagnostics for Kubernetes clusters. Covers DNS, Services, NetworkPolicy, CNI plugins, ingress, and pod-to-pod connectivity.

Inspired by: nicolaka/netshoot, Kubernetes SIG-Network docs, Cilium troubleshooting guide, Calico network policy guide.

## When to Activate

Activate when the user asks about:
- DNS resolution failure, nslookup failing, DNS not working in pod
- Service not reachable, connection refused, connection timeout
- NetworkPolicy blocking traffic, pod cannot reach database
- Pod-to-pod connectivity, inter-namespace communication
- CNI plugin issues, pod CIDR, network overlay
- Kubernetes network debug, network troubleshooting
- Endpoint not ready, service has no endpoints
- kube-dns, CoreDNS issues, DNS 5s timeout
- Ingress not routing, LoadBalancer not accessible

## Script Location

```
skills/k8s-network-debug/diagnose.sh
```

## Usage

```bash
# DNS resolution test in a namespace
bash skills/k8s-network-debug/diagnose.sh -n production --dns

# Service connectivity test
bash skills/k8s-network-debug/diagnose.sh -n production --service my-svc

# NetworkPolicy audit for a pod
bash skills/k8s-network-debug/diagnose.sh -n production --policy --pod my-pod

# Full network scan
bash skills/k8s-network-debug/diagnose.sh -n production --all
```

---

## Troubleshooting Runbook

### Step 1 — Identify Network Symptoms

```bash
# Check all Services and their endpoints
kubectl get svc,endpoints -n <namespace> -o wide

# Services with no ready endpoints (broken service → pod selector mismatch)
kubectl get endpoints -n <namespace> | awk 'NR==1 || $2=="<none>"'

# Check CoreDNS pods are running
kubectl get pods -n kube-system -l k8s-app=kube-dns

# Check CoreDNS logs for errors
kubectl logs -n kube-system -l k8s-app=kube-dns --tail=50
```

---

## Failure Mode: DNS Resolution Failing

**Symptom:** Pods cannot resolve service names or external domains

**The "DNS 5s timeout" bug:** Applications experiencing exactly 5-second DNS delays — caused by conntrack race condition with parallel A + AAAA lookups on older kernels.

**Diagnosis:**

```bash
# Run a netshoot pod for DNS testing (nicolaka/netshoot is the standard debug image)
kubectl run netshoot --rm -it -n <namespace> \
  --image=nicolaka/netshoot -- bash

# Inside netshoot: test DNS
nslookup kubernetes.default.svc.cluster.local
nslookup <service-name>.<namespace>.svc.cluster.local
nslookup <service-name>.<namespace>.svc.cluster.local 10.96.0.10  # CoreDNS ClusterIP

# Test with dig for detailed DNS response
dig @10.96.0.10 <service-name>.<namespace>.svc.cluster.local

# Check /etc/resolv.conf inside pod
kubectl exec <pod> -n <namespace> -- cat /etc/resolv.conf

# Check CoreDNS ConfigMap
kubectl get configmap coredns -n kube-system -o yaml

# CoreDNS logs — look for SERVFAIL, REFUSED, loop detection
kubectl logs -n kube-system -l k8s-app=kube-dns --since=1h | grep -i "error\|fail\|refused"
```

**Fix for DNS 5s timeout:**
```bash
# Edit CoreDNS configmap — set single-stack DNS
kubectl edit configmap coredns -n kube-system
# Add in Corefile: 'autopath @kubernetes' or set 'pods insecure'

# Or disable parallel A+AAAA lookups at pod level via dnsConfig
# Add to pod spec:
# dnsConfig:
#   options:
#   - name: single-request-reopen
#   - name: ndots
#     value: "2"
```

**Fix for NXDOMAIN / cannot resolve:**
```bash
# Verify CoreDNS is running
kubectl rollout restart deployment/coredns -n kube-system

# Check if ndots is set too high (causes slow resolution)
kubectl exec <pod> -n <ns> -- cat /etc/resolv.conf
# options ndots:5 is default — means 5 search domain attempts before external

# Check CoreDNS replicas
kubectl get deployment coredns -n kube-system
kubectl scale deployment coredns -n kube-system --replicas=2
```

---

## Failure Mode: Service Not Reachable

**Symptom:** `connection refused` or `connection timed out` when accessing a Service

**Diagnosis flow:**

```bash
# Step 1: Does the Service exist?
kubectl get svc <service-name> -n <namespace>

# Step 2: Does it have endpoints?
kubectl get endpoints <service-name> -n <namespace>
# If ENDPOINTS shows <none> → pod selector mismatch (most common cause)

# Step 3: Check pod labels vs service selector
kubectl get svc <service-name> -n <namespace> -o jsonpath='{.spec.selector}'
kubectl get pods -n <namespace> --show-labels | grep <expected-label>

# Step 4: Check pod is Running AND Ready
kubectl get pods -n <namespace> -l <key=value>

# Step 5: Test from inside cluster using netshoot
kubectl run netshoot --rm -it -n <namespace> \
  --image=nicolaka/netshoot -- bash
# Inside: curl http://<service-name>.<namespace>.svc.cluster.local:<port>
# Inside: nc -zv <service-name> <port>

# Step 6: Test directly to pod IP (bypass service)
POD_IP=$(kubectl get pod <pod> -n <ns> -o jsonpath='{.status.podIP}')
kubectl run netshoot --rm -it --image=nicolaka/netshoot -- curl http://$POD_IP:<port>

# Step 7: Check kube-proxy / iptables rules on node
kubectl get pods -n kube-system | grep kube-proxy
kubectl logs -n kube-system <kube-proxy-pod> --tail=30
```

**Root cause determination:**
- No endpoints → pod selector mismatch or pods not Ready
- Can reach pod IP but not ClusterIP → kube-proxy / iptables issue
- Can reach ClusterIP but not DNS name → CoreDNS issue
- Timeout from outside cluster but works inside → ingress/LB issue

**Fix selector mismatch:**
```bash
# Check what labels pods actually have
kubectl get pods -n <ns> --show-labels

# Check what selector the service expects
kubectl get svc <svc> -n <ns> -o jsonpath='{.spec.selector}'

# Patch service selector
kubectl patch svc <svc> -n <ns> \
  -p '{"spec":{"selector":{"app":"correct-label-value"}}}'
```

---

## Failure Mode: NetworkPolicy Blocking Traffic

**Symptom:** Pod can't reach another pod/service after NetworkPolicy was applied

**Understanding NetworkPolicy:**
- By default: all traffic allowed (no policies = allow all)
- First NetworkPolicy on a pod: **default deny all** (only specified traffic passes)
- Additive: multiple policies OR together
- Ingress = incoming to pod, Egress = outgoing from pod

**Diagnosis:**

```bash
# List all NetworkPolicies in namespace
kubectl get networkpolicy -n <namespace>
kubectl describe networkpolicy -n <namespace>

# Check what policies apply to a specific pod
POD_LABELS=$(kubectl get pod <pod> -n <ns> --show-labels | tail -1 | awk '{$1=$2=$3=$4=""; print $0}')
kubectl get networkpolicy -n <ns> -o yaml | grep -A10 "podSelector"

# Test connectivity with netshoot
kubectl run netshoot --rm -it --image=nicolaka/netshoot -n <source-ns> -- \
  nc -zv <target-pod-ip> <port>
# or
kubectl run netshoot --rm -it --image=nicolaka/netshoot -n <source-ns> -- \
  curl -m 5 http://<svc>.<target-ns>.svc.cluster.local:<port>

# For Cilium CNI: check policy enforcement
kubectl exec -n kube-system <cilium-pod> -- cilium endpoint list
kubectl exec -n kube-system <cilium-pod> -- cilium policy get

# For Calico CNI: trace policy
kubectl exec -n kube-system <calico-node-pod> -- \
  calicoctl get networkpolicy --namespace=<ns> -o yaml
```

**Common NetworkPolicy patterns:**

```yaml
# Allow all ingress from same namespace
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-same-namespace
  namespace: production
spec:
  podSelector: {}
  ingress:
  - from:
    - podSelector: {}

# Allow specific port from monitoring namespace
spec:
  podSelector:
    matchLabels:
      app: my-app
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: monitoring
    ports:
    - port: 9090
      protocol: TCP

# Allow egress to DNS and specific services
spec:
  podSelector: {}
  egress:
  - ports:  # Allow DNS
    - port: 53
      protocol: UDP
    - port: 53
      protocol: TCP
  - to:
    - podSelector:
        matchLabels:
          app: database
    ports:
    - port: 5432
```

---

## Failure Mode: CNI Issues

**Symptom:** Pods stuck in ContainerCreating, flannel/calico/cilium errors

**Diagnosis:**

```bash
# Check CNI pods
kubectl get pods -n kube-system | grep -E "calico|cilium|flannel|weave|canal"

# Node-level CNI status
kubectl describe node <node> | grep -A5 "Conditions:"

# Check kubelet logs on node for CNI errors
kubectl debug node/<node-name> -it --image=ubuntu -- bash
# Inside: journalctl -u kubelet --since "30min ago" | grep -i cni

# Calico status
kubectl exec -n kube-system <calico-node-pod> -- calico-node -bird-live
kubectl exec -n kube-system <calico-node-pod> -- calicoctl node status

# Cilium status
kubectl exec -n kube-system <cilium-pod> -- cilium status
kubectl exec -n kube-system <cilium-pod> -- cilium connectivity test

# Flannel: check /etc/cni/net.d/ and /var/lib/cni/networks/
kubectl debug node/<node> -it --image=ubuntu -- ls /etc/cni/net.d/
```

---

## Quick Network Testing with Netshoot

`nicolaka/netshoot` is the community-standard Kubernetes network troubleshooting image. Contains: `curl`, `wget`, `dig`, `nslookup`, `nc`, `nmap`, `tcpdump`, `iperf3`, `traceroute`, `mtr`, `iptables`, `ss`, `iproute2`.

```bash
# Launch temporary debug pod (deleted on exit)
kubectl run netshoot --rm -it \
  --image=nicolaka/netshoot \
  -n <target-namespace> \
  -- bash

# Attach to same network namespace as existing pod (K8s 1.23+)
kubectl debug -it <pod> -n <ns> \
  --image=nicolaka/netshoot \
  --target=<container-name>

# Inside netshoot — common tests:
dig <service>.<namespace>.svc.cluster.local
nslookup <service>
curl -v http://<service>:<port>/
nc -zv <host> <port>          # TCP port check
nc -zvu <host> <port>         # UDP port check
tcpdump -i eth0 port <port>   # Packet capture
traceroute <destination>
iperf3 -c <server-pod-ip>     # Bandwidth test
ss -tulnp                      # Socket statistics
```

---

## Service Mesh Conflicts (Istio/Linkerd)

When a service mesh is present, networking works differently:

```bash
# Check if Istio sidecar is injected
kubectl get pod <pod> -n <ns> -o jsonpath='{.spec.containers[*].name}'
# Look for "istio-proxy" in the list

# Istio: check if traffic is being intercepted
kubectl exec <pod> -n <ns> -c istio-proxy -- \
  pilot-agent request GET /stats | grep downstream

# Istio: bypass sidecar for debugging (temporary)
kubectl label namespace <ns> istio-injection=disabled --overwrite
# (delete pod to recreate without sidecar)

# Linkerd: check proxy is injected
kubectl get pod <pod> -n <ns> -o jsonpath='{.spec.containers[*].name}'
# Look for "linkerd-proxy"
```

---

## References

- [nicolaka/netshoot](https://github.com/nicolaka/netshoot) — Network troubleshooting Swiss Army knife
- [Kubernetes: Debugging Services](https://kubernetes.io/docs/tasks/debug/debug-application/debug-service/)
- [Kubernetes: NetworkPolicy](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
- [Cilium Troubleshooting](https://docs.cilium.io/en/stable/operations/troubleshooting/)
- [Calico Troubleshooting](https://docs.tigera.io/calico/latest/operations/troubleshoot/)
- [CoreDNS Troubleshooting](https://coredns.io/plugins/errors/)
