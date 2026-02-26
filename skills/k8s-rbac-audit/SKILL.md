---
name: k8s-rbac-audit
description: Kubernetes RBAC troubleshooting and auditing - permission denied errors, ServiceAccount bindings, Role/ClusterRole analysis, kubectl auth can-i checks, and least-privilege remediation.
metadata:
  emoji: "🔐"
  requires:
    bins: ["kubectl", "bash"]
---

# K8s RBAC Audit — Permission Troubleshooting Runbook

RBAC diagnostic playbook for Kubernetes. Covers permission denied errors, ServiceAccount analysis, Role/ClusterRole binding inspection, and privilege audit.

Inspired by: kubectl-who-can (aquasecurity), rbac-lookup (FairwindsOps), kube-bench (CIS benchmarks), rakkess, Kubernetes RBAC docs.

## When to Activate

Activate when the user asks about:
- Permission denied, forbidden 403, RBAC error
- kubectl auth can-i, check permissions
- ServiceAccount permissions, pod cannot access API
- Role, ClusterRole, RoleBinding, ClusterRoleBinding
- RBAC audit, who can do what
- Least privilege, RBAC hardening
- Service account token, projected volume
- Pod Security Admission, PSA, PSP deprecated
- Operator RBAC, controller permissions
- RBAC for CI/CD, GitHub Actions, ArgoCD

## Troubleshooting Runbook

### Step 1 — Identify the Permission Error

```bash
# Typical RBAC error:
# Error from server (Forbidden): pods is forbidden:
#   User "system:serviceaccount:production:my-app"
#   cannot list resource "pods" in API group ""
#   in the namespace "production"

# Parse the error:
# Subject: system:serviceaccount:production:my-app
# Verb: list
# Resource: pods
# Namespace: production
```

---

## Failure Mode: Pod Cannot Access Kubernetes API

**Symptom:** Application in pod gets 403 Forbidden when calling Kubernetes API

**Diagnosis:**

```bash
# Check the pod's ServiceAccount
kubectl get pod <pod> -n <ns> -o jsonpath='{.spec.serviceAccountName}'

# Check ServiceAccount exists
kubectl get serviceaccount <sa-name> -n <ns>

# Check what roles are bound to this SA
kubectl get rolebindings,clusterrolebindings -A \
  -o json | jq -r --arg sa "<sa-name>" --arg ns "<ns>" '
  .items[] |
  select(
    .subjects[]? |
    (.kind == "ServiceAccount" and .name == $sa and .namespace == $ns)
  ) |
  .metadata.namespace + "/" + .metadata.name + " → " + .roleRef.name'

# Test exactly what the SA can do
kubectl auth can-i list pods \
  --as=system:serviceaccount:<ns>:<sa-name> -n <ns>

kubectl auth can-i --list \
  --as=system:serviceaccount:<ns>:<sa-name> -n <ns>

# All permissions the SA has (requires kubectl 1.26+)
kubectl auth can-i --list \
  --as=system:serviceaccount:<ns>:<sa-name>
```

---

## Checking Permissions with kubectl auth can-i

```bash
# Can current user/SA do something?
kubectl auth can-i get pods -n production
kubectl auth can-i create deployments -n production
kubectl auth can-i delete secrets -n production

# Impersonate a specific user
kubectl auth can-i get pods --as=jane@example.com -n production

# Impersonate a ServiceAccount
kubectl auth can-i get pods \
  --as=system:serviceaccount:production:my-app \
  -n production

# List ALL permissions for a user/SA
kubectl auth can-i --list --as=system:serviceaccount:production:my-app
kubectl auth can-i --list --as=system:serviceaccount:production:my-app -n production

# Check non-resource URLs (e.g., /healthz)
kubectl auth can-i get /healthz --as=system:serviceaccount:production:my-app
```

---

## Inspecting RBAC Resources

```bash
# List all Roles in namespace
kubectl get roles -n <namespace>
kubectl describe role <role-name> -n <namespace>

# List all ClusterRoles
kubectl get clusterroles | grep -v "system:"
kubectl describe clusterrole <name>

# RoleBindings in namespace
kubectl get rolebindings -n <namespace>
kubectl get rolebindings -n <namespace> -o wide  # shows subjects

# ClusterRoleBindings
kubectl get clusterrolebindings
kubectl get clusterrolebindings -o wide | grep -v "system:"

# Who has cluster-admin? (CRITICAL security check)
kubectl get clusterrolebindings -o json | jq -r '
  .items[] |
  select(.roleRef.name == "cluster-admin") |
  "ClusterRoleBinding: " + .metadata.name + "\n" +
  "Subjects: " + ([.subjects[]? | .kind + "/" + .name] | join(", "))'

# Find all bindings for a specific user/SA
kubectl get rolebindings,clusterrolebindings -A \
  -o json | jq -r '.items[] |
  select(.subjects[]? | .name == "my-service-account") |
  .metadata.name'
```

---

## Creating Minimal RBAC (Least Privilege)

**Principle:** Grant only what the workload needs. Use Roles (namespaced) not ClusterRoles unless necessary.

```bash
# Step 1: Create dedicated ServiceAccount
kubectl create serviceaccount my-app -n production

# Step 2: Create Role with minimal permissions
kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: my-app-role
  namespace: production
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["configmaps"]
  resourceNames: ["my-app-config"]  # restrict to specific resource
  verbs: ["get"]
EOF

# Step 3: Bind Role to ServiceAccount
kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: my-app-rolebinding
  namespace: production
subjects:
- kind: ServiceAccount
  name: my-app
  namespace: production
roleRef:
  kind: Role
  apiGroup: rbac.authorization.k8s.io
  name: my-app-role
EOF

# Step 4: Verify
kubectl auth can-i get pods \
  --as=system:serviceaccount:production:my-app \
  -n production
```

---

## RBAC for Common Patterns

### Read-only viewer (namespace)
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: namespace-viewer
rules:
- apiGroups: ["", "apps", "batch"]
  resources: ["pods", "deployments", "services", "jobs", "cronjobs", "configmaps", "endpoints"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["pods/log"]
  verbs: ["get"]
```

### CI/CD deploy role (namespace-scoped)
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: ci-deploy
  namespace: production
rules:
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets"]
  verbs: ["get", "list", "update", "patch"]
- apiGroups: [""]
  resources: ["services", "configmaps"]
  verbs: ["get", "list", "create", "update", "patch"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
```

### Monitoring/Prometheus role
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: prometheus
rules:
- apiGroups: [""]
  resources: ["nodes", "pods", "services", "endpoints"]
  verbs: ["get", "list", "watch"]
- nonResourceURLs: ["/metrics", "/metrics/cadvisor"]
  verbs: ["get"]
```

---

## Security Audit: Finding Over-Privileged Accounts

```bash
# Find all wildcard (*) permissions
kubectl get clusterroles -o json | jq -r '
  .items[] |
  select(.rules[]? | .verbs[] == "*" or .resources[] == "*" or .apiGroups[] == "*") |
  .metadata.name'

# Find accounts with create/delete on sensitive resources
kubectl get clusterroles -o json | jq -r '
  .items[] |
  select(.rules[]? |
    (.resources[] | test("secrets|clusterroles|clusterrolebindings")) and
    (.verbs[] | test("create|delete|update"))
  ) | .metadata.name' | grep -v "^system:"

# ServiceAccounts with token automount (potential risk)
kubectl get serviceaccounts -A \
  -o json | jq -r '.items[] |
  select(.automountServiceAccountToken != false) |
  .metadata.namespace + "/" + .metadata.name' | grep -v "default\|kube"

# Disable token automount for SA (if not needed)
kubectl patch serviceaccount <sa-name> -n <ns> \
  -p '{"automountServiceAccountToken": false}'
```

---

## Third-party RBAC Tools

```bash
# kubectl-who-can: who can do an action?
# Install: https://github.com/aquasecurity/kubectl-who-can
kubectl who-can list pods -n production
kubectl who-can delete secrets

# rbac-lookup: find roles/bindings for a user/SA
# Install: https://github.com/FairwindsOps/rbac-lookup
rbac-lookup my-service-account -n production
rbac-lookup jane@example.com

# rakkess: access matrix for a user
# Install: https://github.com/corneliusweig/rakkess
rakkess --as system:serviceaccount:production:my-app
rakkess --namespace production

# kube-bench: CIS benchmark RBAC checks
# Install: https://github.com/aquasecurity/kube-bench
kubectl apply -f https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml
kubectl logs job/kube-bench | grep -A5 "RBAC"
```

---

## References

- [Kubernetes: RBAC Authorization](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [kubectl-who-can](https://github.com/aquasecurity/kubectl-who-can)
- [rbac-lookup](https://github.com/FairwindsOps/rbac-lookup)
- [rakkess — access matrix](https://github.com/corneliusweig/rakkess)
- [kube-bench CIS benchmarks](https://github.com/aquasecurity/kube-bench)
- [RBAC Good Practices](https://kubernetes.io/docs/concepts/security/rbac-good-practices/)
