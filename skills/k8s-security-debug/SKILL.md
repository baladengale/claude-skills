---
name: k8s-security-debug
description: Kubernetes security troubleshooting - Pod Security Admission levels, OPA Gatekeeper constraints, Falco alerts, audit logs, seccomp/AppArmor profiles, image scanning with Trivy, and supply chain security.
metadata:
  emoji: "🔒"
  requires:
    bins: ["kubectl", "bash"]
---

# K8s Security Debug — Security Troubleshooting Runbook

Security diagnostics for Kubernetes clusters covering policy enforcement (PSA, OPA Gatekeeper), runtime threat detection (Falco), audit logs, workload security contexts, and image vulnerability scanning.

Inspired by: CIS Kubernetes Benchmark, kube-bench, Falco, OPA Gatekeeper, NSA/CISA Kubernetes Hardening Guide, Trivy, kyverno.

## When to Activate

Activate when the user asks about:
- Pod Security Admission (PSA) blocking pods
- OPA Gatekeeper constraint violation
- Falco alert, runtime threat detection
- Kubernetes audit log, API server audit
- seccomp profile, AppArmor profile
- Container running as root, privileged container
- Image scanning, CVE in container image
- Kyverno policy violation
- Supply chain security, SBOM, Cosign
- Network policy security, zero-trust
- Secret scanning, leaked credentials in pod

## Troubleshooting Runbook

### Step 1 — Security Posture Overview

```bash
# Quick security audit: privileged pods, host network, root containers
kubectl get pods -A -o json | jq -r '
  .items[] |
  select(
    .spec.hostNetwork == true or
    .spec.containers[].securityContext.privileged == true or
    .spec.containers[].securityContext.runAsRoot == true or
    (.spec.containers[].securityContext.runAsUser // 1) == 0
  ) |
  .metadata.namespace + "/" + .metadata.name + " (SECURITY RISK)"'

# Pods with no securityContext
kubectl get pods -A -o json | jq -r '
  .items[] |
  select(.spec.containers[].securityContext == null) |
  .metadata.namespace + "/" + .metadata.name'
```

---

## Failure Mode: Pod Security Admission (PSA) Blocking Pod

**PSA** replaces PodSecurityPolicy (deprecated). It enforces security standards at the namespace level.

**Three policy levels:**
- `privileged` — no restrictions
- `baseline` — prevents known privilege escalations
- `restricted` — hardened, follows best practices (no root, no hostPath, seccomp required)

**Three modes:**
- `enforce` — reject non-compliant pods
- `audit` — allow but log violation
- `warn` — allow but warn

```bash
# Check PSA labels on namespace
kubectl get namespace <ns> --show-labels | grep "pod-security"

# Labels format:
# pod-security.kubernetes.io/enforce: restricted
# pod-security.kubernetes.io/warn: baseline
# pod-security.kubernetes.io/audit: baseline

# Audit what would fail in restricted mode (without enforcing)
kubectl label namespace <ns> \
  pod-security.kubernetes.io/audit=restricted \
  pod-security.kubernetes.io/warn=restricted

# Check audit log for PSA violations
kubectl logs -n kube-system kube-apiserver-<node> | \
  grep "PodSecurity"

# Test if a pod spec would be admitted
kubectl run test --dry-run=server --image=nginx -n <ns>

# Apply PSA to namespace
kubectl label namespace <ns> \
  pod-security.kubernetes.io/enforce=restricted \
  pod-security.kubernetes.io/enforce-version=latest

# COMMON FIX: add security context to make pod compliant
# For restricted level:
# spec.securityContext.runAsNonRoot: true
# spec.securityContext.seccompProfile.type: RuntimeDefault
# containers.securityContext.allowPrivilegeEscalation: false
# containers.securityContext.capabilities.drop: [ALL]
```

**Compliant pod security context:**
```yaml
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    runAsGroup: 3000
    fsGroup: 2000
    seccompProfile:
      type: RuntimeDefault
  containers:
  - name: app
    securityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop: ["ALL"]
```

---

## Failure Mode: OPA Gatekeeper Constraint Violation

**Symptom:** Pod rejected with `admission webhook denied`, message contains policy name

```bash
# List Gatekeeper pods
kubectl get pods -n gatekeeper-system

# List constraint templates (defines policy logic in Rego)
kubectl get constrainttemplates

# List constraints (instances with parameters)
kubectl get constraints

# Check violation count per constraint
kubectl get constraints -o json | jq -r '
  .items[] |
  .metadata.name + ": " + (.status.totalViolations | tostring) + " violations"'

# See all violations for a constraint
kubectl describe constraint <name>
kubectl get constraint <name> -o jsonpath='{.status.violations[*]}'

# Check Gatekeeper audit logs (re-audits all existing resources)
kubectl logs -n gatekeeper-system deployment/gatekeeper-audit --tail=30

# Webhook configuration
kubectl get validatingwebhookconfiguration gatekeeper-validating-webhook-configuration -o yaml

# Dry-run: would a resource be blocked?
kubectl apply -f my-pod.yaml --dry-run=server 2>&1 | grep "gatekeeper\|constraint"

# Temporarily disable a constraint (set enforcementAction to dryrun)
kubectl patch constraint <name> \
  --type='json' \
  -p='[{"op":"replace","path":"/spec/enforcementAction","value":"dryrun"}]'

# Common constraint types:
# k8srequiredlabels — pods must have specific labels
# k8scontainerlimits — containers must have resource limits
# k8sallowedrepos — images must come from approved registries
# k8snolatestimages — disallow :latest tags
# k8spodsecuritypolicies — replicate PSP behavior
```

---

## Failure Mode: Kyverno Policy Blocking Workloads

```bash
# Kyverno pods
kubectl get pods -n kyverno

# List policies
kubectl get clusterpolicy
kubectl get policy -A

# Check policy violations
kubectl get policyreport -A
kubectl get clusterpolicyreport

# Policy violation details
kubectl describe policyreport -n <ns> <name>

# Dry-run against a resource
kubectl apply -f my-pod.yaml --dry-run=server 2>&1

# Policy audit mode (don't block, just report)
kubectl get clusterpolicy <name> -o yaml | grep validationFailureAction
# Enforce = block, Audit = report only
# Switch to audit: kubectl patch clusterpolicy <name> --type=merge -p '{"spec":{"validationFailureAction":"audit"}}'
```

---

## Falco — Runtime Threat Detection

Falco detects anomalous behavior at runtime (shell in container, network connection from container, file access).

```bash
# Falco pods
kubectl get pods -n falco
kubectl get pods -A | grep falco

# Falco alerts (this is where real-time detections appear)
kubectl logs -n falco daemonset/falco --tail=30
kubectl logs -n falco daemonset/falco -f  # stream alerts

# Example Falco alert format:
# {"output":"14:31:01.234 Warning Sensitive file opened for reading by non-trusted program
#   (user=root command=cat /etc/shadow file=/etc/shadow)","priority":"Warning","rule":"Read sensitive file trusted after startup"}

# Falco rules
kubectl get configmap falco-rules -n falco -o yaml

# Common Falco rules that fire frequently:
# "Shell spawned in container" → exec into pod
# "Write below binary dir" → writing to /bin, /usr/bin
# "Read sensitive file" → reading /etc/shadow, /etc/passwd
# "Network tool launched" → nmap, netcat, tcpdump in container
# "Outbound connection to C2 IP" → if threat intel is configured

# Tune noisy rules (add to falco_rules.local.yaml)
# - macro: trusted_containers
#   condition: container.image.repository in (my-trusted-image)

# Falco Sidekick (forwards alerts to Slack, PagerDuty, etc.)
kubectl get pods -n falco | grep sidekick
kubectl logs -n falco deployment/falco-falcosidekick --tail=20
```

---

## Kubernetes Audit Logs

The API server can log every request — critical for incident investigation.

```bash
# Check if audit logging is enabled
cat /etc/kubernetes/manifests/kube-apiserver.yaml | grep -E "audit-log|audit-policy"

# View audit log
tail -f /var/log/kubernetes/audit/audit.log | jq .

# Filter for specific user/SA activity
cat /var/log/kubernetes/audit/audit.log | jq -r \
  'select(.user.username == "system:serviceaccount:production:my-app") |
   .verb + " " + .objectRef.resource + "/" + .objectRef.name'

# Find secrets accessed
cat /var/log/kubernetes/audit/audit.log | jq -r \
  'select(.objectRef.resource == "secrets") |
   .user.username + " " + .verb + " " + .objectRef.namespace + "/" + .objectRef.name'

# Audit policy example (log secrets at RequestResponse level)
# /etc/kubernetes/audit-policy.yaml:
# rules:
# - level: RequestResponse
#   resources:
#   - group: ""
#     resources: ["secrets"]
```

---

## Image Scanning with Trivy

```bash
# Install Trivy
# https://trivy.dev/latest/getting-started/installation/

# Scan an image for vulnerabilities
trivy image <registry>/<image>:<tag>
trivy image --severity CRITICAL,HIGH nginx:latest

# Scan running containers in cluster
trivy k8s --report all

# Scan specific namespace
trivy k8s --namespace production --report all

# SBOM generation
trivy image --format cyclonedx --output sbom.json nginx:latest

# Scan Kubernetes manifests / IaC
trivy config ./k8s-manifests/

# In-cluster operator: Trivy Operator
kubectl get pods -n trivy-system
kubectl get vulnerabilityreports -A
kubectl get configauditreports -A
```

---

## seccomp and AppArmor

```bash
# Check seccomp profile type on a pod
kubectl get pod <pod> -n <ns> \
  -o jsonpath='{.spec.securityContext.seccompProfile.type}'
# RuntimeDefault = use container runtime's default seccomp profile
# Localhost = custom profile from node
# Unconfined = no seccomp (risky)

# AppArmor annotation (per container)
kubectl get pod <pod> -n <ns> \
  -o jsonpath='{.metadata.annotations}'
# container.apparmor.security.beta.kubernetes.io/<container>: runtime/default

# Check AppArmor on node
kubectl debug node/<node> -it --image=ubuntu
# chroot /host
# aa-status | grep docker-default

# CIS Benchmark: run kube-bench
kubectl apply -f https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml
kubectl logs job/kube-bench | grep -E "FAIL|WARN" | head -20
```

---

## References

- [Kubernetes: Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
- [OPA Gatekeeper](https://open-policy-agent.github.io/gatekeeper/)
- [Kyverno](https://kyverno.io/docs/)
- [Falco](https://falco.org/docs/)
- [Trivy](https://trivy.dev/)
- [kube-bench (CIS benchmarks)](https://github.com/aquasecurity/kube-bench)
- [NSA/CISA Kubernetes Hardening Guide](https://media.defense.gov/2022/Aug/29/2003066362/-1/-1/0/CTR_KUBERNETES_HARDENING_GUIDANCE_1.2_20220829.PDF)
