---
name: k8s-operator-debug
description: Kubernetes operator troubleshooting - OLM lifecycle, CRD version conflicts, operator reconciliation loops, webhook failures, controller-runtime errors, finalizer deadlocks, and operator SDK debugging.
metadata:
  emoji: "⚙️"
  requires:
    bins: ["kubectl", "bash"]
---

# K8s Operator Debug — Operator Troubleshooting Runbook

Diagnostics for Kubernetes operators built on controller-runtime/operator-sdk, OLM-managed operators, CRD lifecycle issues, and admission webhook failures.

Inspired by: operator-sdk docs, controller-runtime source, OLM docs, Kubebuilder book, CNCF operator whitepaper, Crossplane troubleshooting guide.

## When to Activate

Activate when the user asks about:
- Operator reconciliation loop, controller stuck
- CRD not found, CRD version conflict, conversion webhook
- Finalizer deadlock, object stuck in terminating
- Admission webhook failure, validating webhook
- OLM ClusterServiceVersion, InstallPlan failed
- Operator SDK, Kubebuilder, controller-runtime
- Custom Resource (CR) stuck, CR not reconciled
- Leader election failure, operator restarting
- Crossplane composition, provider issues
- ArgoCD application controller, FluxCD kustomize-controller

## Script Location

```
skills/k8s-operator-debug/diagnose.sh
```

## Usage

```bash
# Audit all CRDs and operator health
bash skills/k8s-operator-debug/diagnose.sh --all

# Check specific operator deployment
bash skills/k8s-operator-debug/diagnose.sh --operator my-operator -n my-operator-ns

# Find terminating objects (finalizer deadlocks)
bash skills/k8s-operator-debug/diagnose.sh --terminating

# Check webhooks
bash skills/k8s-operator-debug/diagnose.sh --webhooks
```

---

## Troubleshooting Runbook

### How Operators Work

```
Custom Resource (CR) created/updated/deleted
         ↓
API Server validates via Admission Webhook (optional)
         ↓
Controller watches for CR changes via List/Watch
         ↓
Reconcile() called → operator reads CR, makes cluster state match desired state
         ↓
Status subresource updated with current state
         ↓
Requeue if needed (for periodic reconciliation)
```

### Step 1 — Operator Pod Health

```bash
# Find operator pods (usually in dedicated namespace)
kubectl get pods -A | grep -E "operator|controller|manager"

# Check operator deployment
kubectl get deployment -n <operator-namespace>
kubectl describe deployment <operator-name> -n <operator-namespace>

# Operator logs — the most important source
kubectl logs -n <operator-ns> deployment/<operator> --tail=50
kubectl logs -n <operator-ns> deployment/<operator> --follow

# Operator logs with error filter
kubectl logs -n <operator-ns> deployment/<operator> --tail=100 | \
  grep -iE "error|fail|panic|warn"

# Check leader election (operators elect a leader to avoid split-brain)
kubectl get lease -n <operator-ns>
kubectl describe lease <lease-name> -n <operator-ns>
```

---

## Failure Mode: Custom Resource Not Reconciling

**Symptom:** Applied a CR but nothing happens — operator doesn't respond

```bash
# Check if the CR was created
kubectl get <cr-kind> -n <namespace>
kubectl describe <cr-kind> <name> -n <namespace>

# Check CR status conditions
kubectl get <cr-kind> <name> -n <namespace> \
  -o jsonpath='{.status.conditions}' | jq .

# Check events for the CR
kubectl get events -n <namespace> \
  --field-selector="involvedObject.name=<cr-name>" \
  --sort-by='.lastTimestamp'

# Operator might be crashing — check for restarts
kubectl get pods -n <operator-ns> | grep -v "1/1\|2/2"

# Check if CRD is installed
kubectl get crd | grep <cr-kind>

# Check API group/version
kubectl api-resources | grep <cr-kind>

# Check operator RBAC — can it read/write the CRD?
kubectl auth can-i get <cr-plural> \
  --as=system:serviceaccount:<operator-ns>:<operator-sa>

# Check operator watches the right namespace
kubectl get deployment <operator> -n <operator-ns> -o yaml | grep -A5 "args:"
# Look for: --watch-namespace or WATCH_NAMESPACE env var
```

---

## Failure Mode: Object Stuck in Terminating (Finalizer Deadlock)

**Symptom:** `kubectl delete` runs but object stays in `Terminating` forever

**Root cause:** Kubernetes won't delete an object until all finalizers are removed. The operator responsible for removing the finalizer is gone/broken.

```bash
# Check which finalizers are blocking deletion
kubectl get <resource> <name> -n <ns> \
  -o jsonpath='{.metadata.finalizers}'

# Identify the operator responsible (finalizer name usually gives a hint)
# e.g., "kopf.zalando.org/KopfFinalizerMarker" → kopf-based operator
# e.g., "kubernetes.io/pvc-protection" → PVC protection

# Option 1: Fix the operator (preferred)
kubectl rollout restart deployment/<operator> -n <operator-ns>
# Wait for operator to process the finalizer

# Option 2: Force remove finalizer (only when operator is gone for good)
kubectl patch <resource> <name> -n <ns> \
  -p '{"metadata":{"finalizers":[]}}' \
  --type=merge
# Object will delete immediately

# For namespaces stuck in Terminating (common when operator CRDs are deleted first)
kubectl get namespace <ns> -o json | \
  jq '.spec.finalizers = []' | \
  kubectl replace --raw "/api/v1/namespaces/<ns>/finalize" -f -
```

---

## Failure Mode: Admission Webhook Rejecting Resources

**Symptom:** `kubectl apply` fails with `admission webhook denied` or `Internal Server Error`

```bash
# List all admission webhooks
kubectl get validatingwebhookconfigurations
kubectl get mutatingwebhookconfigurations

# Describe the webhook (shows failurePolicy and rules)
kubectl describe validatingwebhookconfiguration <name>
kubectl describe mutatingwebhookconfiguration <name>

# Key fields:
# failurePolicy: Fail → webhook failure blocks the request (dangerous if webhook is down)
# failurePolicy: Ignore → webhook failure is ignored
# timeoutSeconds: default 10s

# Check if webhook service is available
kubectl get svc -n <webhook-ns>
kubectl get endpoints <webhook-svc> -n <webhook-ns>

# Test webhook endpoint directly
kubectl exec <pod> -- curl -k https://<webhook-svc>.<ns>.svc.cluster.local:<port>/validate

# Webhook logs (it's usually the operator pod)
kubectl logs -n <operator-ns> deployment/<operator> | grep -i "webhook\|admit"

# EMERGENCY: disable a blocking webhook (temporary)
kubectl delete validatingwebhookconfiguration <name>
# OR patch failurePolicy to Ignore
kubectl patch validatingwebhookconfiguration <name> \
  --type='json' \
  -p='[{"op":"replace","path":"/webhooks/0/failurePolicy","value":"Ignore"}]'
```

**Common webhook issues:**
- TLS cert expired → webhook returns TLS error → Fail policy blocks all resources
- Webhook pod crashed → timeout → Fail policy blocks resources
- caBundle outdated → TLS verification fails → fix with cert-manager injection

---

## Failure Mode: CRD Version Conflict / Conversion Webhook

**Symptom:** Error `no kind is registered for the type` or `no match for kind`

```bash
# Check CRD installed versions
kubectl get crd <crd-name> -o jsonpath='{.spec.versions[*].name}'

# Check CRD storage version
kubectl get crd <crd-name> -o jsonpath='{.status.storedVersions}'

# If upgrading operator: check for conversion webhook
kubectl get crd <crd-name> -o yaml | grep -A10 "conversion:"

# List all objects of old version (need migration)
kubectl get <resource> -A -o yaml | grep "apiVersion: <group>/<old-version>"

# CRD validation schema errors
kubectl apply -f my-cr.yaml 2>&1 | grep -i "validation\|schema\|invalid"

# Check CRD conditions
kubectl describe crd <name> | grep -A10 "Conditions:"
```

---

## Failure Mode: Reconciliation Loop / Operator Thrashing

**Symptom:** Operator logs show the same reconcile running repeatedly, high CPU on operator pod

```bash
# Count reconciliation rate
kubectl logs -n <operator-ns> deployment/<operator> | \
  grep -c "reconcile"

# Check if operator is requeueing on every change it makes
# Operators should NOT trigger their own reconcile by watching resources they modify
# Look for: "Reconcile loop detected" or rapid successive logs

# Check controller metrics (if prometheus is installed)
# rate(controller_runtime_reconcile_total{result="success"}[1m])
# rate(controller_runtime_reconcile_total{result="error"}[1m])

# Watch reconcile events
kubectl logs -n <operator-ns> deployment/<operator> -f | \
  grep -E "reconcile|Reconcile|requeue"
```

**Fix common reconciliation loop causes:**
- Don't set `Status` in a way that triggers a re-watch on the same resource
- Use `ResourceVersion` check before updating
- Use predicates to filter events that don't require reconciliation
- Add `sigs.k8s.io/controller-runtime` generation-changed predicate

---

## Operator Status Conditions Pattern

Well-behaved operators set `.status.conditions` on their CRs. Check these:

```bash
# Get status conditions (standard pattern for all CRDs)
kubectl get <cr-kind> <name> -n <ns> \
  -o jsonpath='{range .status.conditions[*]}{.type}{": "}{.status}{" — "}{.message}{"\n"}{end}'

# Common condition types:
# Ready: True/False
# Available: True/False
# Progressing: True/False
# Degraded: True/False (some use this)
# Synced: True/False (Crossplane, FluxCD)

# For FluxCD
kubectl get kustomizations -A
kubectl describe kustomization <name> -n flux-system

# For Crossplane
kubectl get managed  # all managed resources
kubectl describe <managed-resource> <name>
```

---

## OLM Operator Lifecycle

```bash
# Full OLM object chain for an installed operator:
# CatalogSource → PackageManifest → Subscription → InstallPlan → CSV → Deployment

# Check the full chain
kubectl get catalogsource -n openshift-marketplace   # OCP
kubectl get catalogsource -n olm                     # vanilla OLM

kubectl get packagemanifest -n olm | grep <operator-name>

kubectl get subscription -n <namespace>

kubectl get installplan -n <namespace>
# Approve pending install plan:
kubectl patch installplan <name> -n <namespace> \
  --type=json -p='[{"op":"replace","path":"/spec/approved","value":true}]'

kubectl get csv -n <namespace>
# If CSV is Failed:
kubectl describe csv <name> -n <namespace>
kubectl logs -n <namespace> deployment/<operator> | tail -30
```

---

## Debugging with Controller-Runtime

```bash
# Increase operator log verbosity (if using zap logger)
kubectl set env deployment/<operator> -n <ns> ZAPLOGLEVEL=debug

# Or patch args:
kubectl edit deployment <operator> -n <ns>
# Add: args: ["--zap-log-level=debug"]

# Common controller-runtime log entries:
# "Starting EventSource" → controller starting to watch a resource
# "Starting Controller" → controller manager ready
# "Reconciling" → reconcile called for an object
# "Successfully reconciled" → reconcile completed OK
# "Requeue after" → scheduled requeue
```

---

## References

- [Kubebuilder Book](https://book.kubebuilder.io/)
- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
- [Operator SDK Docs](https://sdk.operatorframework.io/docs/)
- [OLM Concepts](https://olm.operatorframework.io/docs/concepts/)
- [CNCF Operator Whitepaper](https://github.com/cncf/tag-app-delivery/blob/main/operator-wg/whitepaper/Operator-WhitePaper_v1-0.md)
