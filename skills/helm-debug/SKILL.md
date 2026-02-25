---
name: helm-debug
description: Helm chart troubleshooting - failed releases, upgrade errors, rollback procedures, diff comparison, hook failures, chart validation, and Helmfile/ArgoCD sync issues.
metadata:
  emoji: "⛵"
  requires:
    bins: ["helm", "kubectl", "bash"]
---

# Helm Debug — Chart Release Troubleshooting Runbook

Systematic Helm release diagnostics covering installation failures, upgrade errors, stuck releases, hook debugging, and rollback procedures.

Inspired by: Helm official docs, helm-diff plugin, helmfile, Artifact Hub best practices, ArgoCD Helm integration docs.

## When to Activate

Activate when the user asks about:
- Helm install failed, helm upgrade failed
- Helm release stuck in pending-install, pending-upgrade, pending-rollback
- Helm rollback, helm history, revert deployment
- Helm hook failure, pre-install hook failed
- Helm diff, compare chart versions
- Chart validation, helm lint, helm template
- Helmfile sync failed, helmfile apply
- ArgoCD Helm application out of sync
- Helm secret decryption, helm-secrets, SOPS
- Chart dependency update, helm repo update

## Script Location

```
skills/helm-debug/diagnose.sh
```

## Usage

```bash
# List all releases with status
bash skills/helm-debug/diagnose.sh

# Inspect specific failed release
bash skills/helm-debug/diagnose.sh --release my-app --namespace production

# Show release history
bash skills/helm-debug/diagnose.sh --release my-app --namespace production --history

# Rollback to previous version
bash skills/helm-debug/diagnose.sh --release my-app --namespace production --rollback
```

---

## Troubleshooting Runbook

### Step 1 — Check All Release Statuses

```bash
# All releases across all namespaces
helm list -A

# Only failed/pending releases
helm list -A --failed
helm list -A --pending

# Specific namespace
helm list -n <namespace>

# Output with timestamps and chart versions
helm list -A -o json | jq -r '.[] | [.name, .namespace, .status, .chart, .updated] | @tsv'
```

---

## Failure Mode: Release Stuck in pending-install / pending-upgrade

**Symptom:** `helm list` shows `pending-install` or `pending-upgrade` — Helm is "locked"

**Root cause:** A previous install/upgrade attempt was interrupted without cleanup (process killed, context switch, network disconnect).

**Diagnosis:**

```bash
# Check release status
helm status <release> -n <namespace>

# See release history
helm history <release> -n <namespace>

# Check if Helm secret exists (Helm v3 stores state as Secrets)
kubectl get secrets -n <namespace> | grep "sh.helm.release"
kubectl get secret sh.helm.release.v1.<release>.v<N> -n <namespace> -o yaml
```

**Remediation:**
```bash
# Option 1: Delete the stuck release and reinstall
helm delete <release> -n <namespace>
helm install <release> <chart> -n <namespace> -f values.yaml

# Option 2: Force upgrade (skips stuck state)
helm upgrade --install <release> <chart> \
  -n <namespace> \
  -f values.yaml \
  --atomic \
  --cleanup-on-fail \
  --timeout 5m

# Option 3: Manually mark old secret as superseded (advanced)
kubectl patch secret sh.helm.release.v1.<release>.v<N> \
  -n <namespace> \
  --type='json' \
  -p='[{"op":"replace","path":"/data/status","value":"c3VwZXJzZWRlZA=="}]'
# Note: "c3VwZXJzZWRlZA==" is base64 of "superseded"
```

---

## Failure Mode: Upgrade Failed / UPGRADE FAILED

**Symptom:** `Error: UPGRADE FAILED: ...`

**Common causes:**
1. Invalid values (type mismatch, required field missing)
2. Resource conflict (immutable field change, e.g., changing a Deployment selector)
3. Pre-upgrade hook failure
4. Timeout (default 5m)
5. Pod readiness failure (if using `--wait`)

**Diagnosis:**

```bash
# See exact error message
helm upgrade <release> <chart> -n <namespace> -f values.yaml 2>&1

# Dry-run to validate values and templates
helm upgrade <release> <chart> -n <namespace> -f values.yaml --dry-run

# Render templates to inspect manifests
helm template <release> <chart> -f values.yaml | kubectl diff -f -

# Check hook pods (if hook failed)
kubectl get pods -n <namespace> -l "helm.sh/chart" | grep -E "hook|pre-|post-"
kubectl logs -n <namespace> <hook-pod-name>

# Check events
kubectl get events -n <namespace> --sort-by='.lastTimestamp' | tail -20

# Helm debug output
helm upgrade <release> <chart> -n <namespace> -f values.yaml --debug 2>&1 | head -100
```

**Remediation:**
```bash
# Rollback to last good version
helm rollback <release> -n <namespace>

# Rollback to specific version
helm rollback <release> <revision-number> -n <namespace>

# Cleanup failed resources and retry
helm upgrade <release> <chart> -n <namespace> -f values.yaml \
  --cleanup-on-fail \
  --atomic \
  --timeout 10m

# Fix immutable field conflict (e.g., selector change)
# Must delete the resource first
kubectl delete deployment <name> -n <namespace>
helm upgrade <release> <chart> -n <namespace> -f values.yaml
```

---

## Helm Diff — Compare Before Upgrading

`helm-diff` plugin shows what will change before applying. Essential for production upgrades.

```bash
# Install helm-diff plugin
helm plugin install https://github.com/databus23/helm-diff

# Diff current vs new chart version
helm diff upgrade <release> <chart> -n <namespace> -f values.yaml

# Diff with new values file
helm diff upgrade <release> <chart> -n <namespace> \
  -f values.yaml \
  -f values-prod.yaml

# Diff between two revisions
helm diff revision <release> <rev1> <rev2> -n <namespace>

# Show only changed resources
helm diff upgrade <release> <chart> -n <namespace> -f values.yaml \
  --show-secrets=false \
  --normalize-manifests
```

---

## Chart Validation

```bash
# Lint chart (check for syntax errors and best practices)
helm lint ./my-chart
helm lint ./my-chart -f values.yaml

# Render templates (no cluster needed)
helm template my-release ./my-chart -f values.yaml

# Render specific template
helm template my-release ./my-chart -f values.yaml -s templates/deployment.yaml

# Validate rendered templates against cluster
helm template my-release ./my-chart -f values.yaml | kubectl apply --dry-run=client -f -
helm template my-release ./my-chart -f values.yaml | kubectl apply --dry-run=server -f -

# Check chart values
helm show values <chart>
helm show values <chart> > default-values.yaml

# Verify chart dependencies
helm dependency list ./my-chart
helm dependency update ./my-chart
```

---

## Hook Debugging

Helm hooks run as Jobs/Pods at lifecycle points:
- `pre-install`, `post-install`
- `pre-upgrade`, `post-upgrade`
- `pre-delete`, `post-delete`
- `pre-rollback`, `post-rollback`
- `test`

```bash
# List hook resources
kubectl get jobs,pods -n <namespace> -l "helm.sh/chart"

# Check hook annotations on resources
kubectl get jobs -n <namespace> -o json | \
  jq -r '.items[] | select(.metadata.annotations."helm.sh/hook" != null) |
  .metadata.name + " → " + .metadata.annotations."helm.sh/hook"'

# Get hook pod logs
kubectl get pods -n <namespace> \
  -l "helm.sh/chart" \
  -o name | xargs -I{} kubectl logs {} -n <namespace>

# Run helm tests
helm test <release> -n <namespace>
helm test <release> -n <namespace> --logs

# Hook cleanup policy (hook-delete-policy annotation)
# hook-succeeded — delete after success (default)
# hook-failed — delete after failure
# before-hook-creation — delete before next hook run
```

---

## Release History and Rollback

```bash
# Full release history
helm history <release> -n <namespace>

# Show values used in a specific revision
helm get values <release> -n <namespace> --revision <N>

# Show manifests from a specific revision
helm get manifest <release> -n <namespace> --revision <N>

# Rollback to previous version
helm rollback <release> -n <namespace>

# Rollback to specific revision
helm rollback <release> 3 -n <namespace>

# Rollback with wait (waits for pods to be ready)
helm rollback <release> -n <namespace> --wait --timeout 5m

# Uninstall but keep history
helm uninstall <release> -n <namespace> --keep-history
```

---

## Helmfile Integration

```bash
# Sync all releases defined in helmfile.yaml
helmfile sync

# Diff before sync
helmfile diff

# Apply (only changed releases)
helmfile apply

# Sync specific release
helmfile -l name=my-release sync

# Debug a specific environment
helmfile -e production diff

# Destroy all releases (DANGER: use with caution)
helmfile destroy
```

---

## ArgoCD + Helm

```bash
# Check ArgoCD app sync status
argocd app list
argocd app get <app-name>

# Force sync
argocd app sync <app-name>

# Sync with Helm parameter override
argocd app set <app-name> -p image.tag=v2.0.0
argocd app sync <app-name>

# Check rendered Helm manifests
argocd app manifests <app-name>

# View app diff
argocd app diff <app-name>
```

---

## Common Helm Errors Reference

| Error | Cause | Fix |
|-------|-------|-----|
| `context deadline exceeded` | Timeout during --wait | Increase --timeout or fix pod readiness |
| `rendered manifests contain a resource that already exists` | Resource not owned by this release | Add `--force` or adopt the resource |
| `cannot patch ... field is immutable` | Changing selector labels | Delete resource first, then upgrade |
| `UPGRADE FAILED: another operation is in progress` | Release is locked | Delete pending secret or rollback |
| `Error: no repositories found` | Helm repo not added | `helm repo add` the chart repository |
| `coalesce.go: warning: destination for key is not a table` | Values type mismatch | Fix values.yaml structure |

---

## References

- [Helm: Debugging Charts](https://helm.sh/docs/chart_template_guide/debugging/)
- [helm-diff plugin](https://github.com/databus23/helm-diff)
- [helmfile](https://helmfile.readthedocs.io/)
- [Helm Chart Best Practices](https://helm.sh/docs/chart_best_practices/)
- [ArgoCD Helm Integration](https://argo-cd.readthedocs.io/en/stable/user-guide/helm/)
