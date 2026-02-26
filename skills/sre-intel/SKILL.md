---
name: sre-intel
description: SRE observability intelligence - queries Prometheus for HTTP error rates and latency SLIs, Alertmanager for firing alerts, and computes 30-day SLO compliance with error budget burn rates.
metadata:
  emoji: "📊"
  requires:
    bins: ["curl", "bash"]
---

# SRE Intel — Prometheus & Alertmanager Intelligence

Plain-language runbook for SRE observability. Claude queries Prometheus and Alertmanager APIs directly, surfaces error rates, latency percentiles, SLO compliance, firing alerts, and error budget burn.

Inspired by open-source SRE tooling patterns (Google SRE Book, OpenSLO, sloth, pyrra).

## When to Activate

Activate when the user asks about:
- SRE dashboard, SLO status, error budget, SLI
- Prometheus metrics, alert status, alertmanager
- Service error rates, latency p99, p95, p50
- Firing alerts, alert summary, on-call intelligence
- Error budget burn, SLO compliance, availability
- Incident overview, service health scorecard

---

## Configuration

Set these environment variables (or provide the URLs directly):

```bash
export PROMETHEUS_URL=http://prometheus:9090      # default: http://localhost:9090
export ALERTMANAGER_URL=http://alertmanager:9093  # default: http://localhost:9093
```

---

## Step 1 — Check Firing Alerts

Query Alertmanager for all active alerts:

```bash
# Alertmanager v2 API
curl -s "${ALERTMANAGER_URL:-http://localhost:9093}/api/v2/alerts" | \
  jq -r '.[] | [.labels.severity, .state, (.labels.alertname // "unknown"), (.annotations.summary // "")] | @tsv' | \
  sort -t$'\t' -k1,1

# v1 fallback (older Alertmanager)
curl -s "${ALERTMANAGER_URL:-http://localhost:9093}/api/v1/alerts" | \
  jq -r '.data[] | [.labels.severity, .state, (.labels.alertname // "unknown"), (.annotations.summary // "")] | @tsv' | \
  sort -t$'\t' -k1,1
```

**Present firing alerts in this format:**
```
=== Firing Alerts ===
Critical: 2   Warning: 5   Total: 7

SEVERITY   STATE    ALERT                           SINCE
critical   firing   HighErrorRate                   2024-01-15 10:23:01
critical   firing   PodCrashLooping                 2024-01-15 09:45:12
warning    firing   HighMemoryUsage                 2024-01-15 11:01:00
...
```

**Severity priority:** critical → warning → info

---

## Step 2 — HTTP Error Rates Per Service

Run these PromQL queries against Prometheus:

```bash
PROM="${PROMETHEUS_URL:-http://localhost:9090}"
RANGE="1h"   # configurable: 1h, 6h, 24h

# Error rate (% of 5xx over total requests) per service
curl -sG "$PROM/api/v1/query" \
  --data-urlencode "query=100 * sum(rate(http_requests_total{status=~\"5..\"}[${RANGE}])) by (job) / sum(rate(http_requests_total[${RANGE}])) by (job)" | \
  jq -r '.data.result[] | [.metric.job, .value[1]] | @tsv'

# Request rate (req/s) per service
curl -sG "$PROM/api/v1/query" \
  --data-urlencode "query=sum(rate(http_requests_total[${RANGE}])) by (job)" | \
  jq -r '.data.result[] | [.metric.job, .value[1]] | @tsv'
```

---

## Step 3 — Latency SLIs (P50 / P95 / P99)

```bash
PROM="${PROMETHEUS_URL:-http://localhost:9090}"
RANGE="1h"

# P50 latency (seconds) per service
curl -sG "$PROM/api/v1/query" \
  --data-urlencode "query=histogram_quantile(0.50, sum(rate(http_request_duration_seconds_bucket[${RANGE}])) by (le, job))" | \
  jq -r '.data.result[] | [.metric.job, (.value[1] | tonumber * 1000 | round | tostring + "ms")] | @tsv'

# P95 latency
curl -sG "$PROM/api/v1/query" \
  --data-urlencode "query=histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[${RANGE}])) by (le, job))" | \
  jq -r '.data.result[] | [.metric.job, (.value[1] | tonumber * 1000 | round | tostring + "ms")] | @tsv'

# P99 latency
curl -sG "$PROM/api/v1/query" \
  --data-urlencode "query=histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[${RANGE}])) by (le, job))" | \
  jq -r '.data.result[] | [.metric.job, (.value[1] | tonumber * 1000 | round | tostring + "ms")] | @tsv'
```

---

## Step 4 — 30-Day SLO Compliance

```bash
PROM="${PROMETHEUS_URL:-http://localhost:9090}"

# 30-day availability % per service
curl -sG "$PROM/api/v1/query" \
  --data-urlencode "query=100 * (1 - sum(rate(http_requests_total{status=~\"5..\"}[30d])) by (job) / sum(rate(http_requests_total[30d])) by (job))" | \
  jq -r '.data.result[] | [.metric.job, (.value[1] | tonumber)] | @tsv'
```

---

## Step 5 — Calculate Error Budget and Burn Rate

**Formula:**
```
error_budget_remaining = (allowed_errors - actual_errors) / allowed_errors × 100

where:
  allowed_errors = 100 - slo_target  (e.g., 0.1% for 99.9% SLO)
  actual_errors  = 100 - availability_30d
```

**Burn Rate Classification:**
| Budget Remaining | Status |
|------------------|--------|
| > 70% | 🟢 healthy |
| 30–70% | 🟡 moderate |
| 10–30% | 🟠 fast |
| 1–10% | 🔴 critical |
| ≤ 0% | 🔴 exhausted |

**SLO Reference Table:**

| SLO Target | Error Budget (30d) | Allowed Downtime |
|------------|-------------------|------------------|
| 99.9% | 43.2 min/month | 0.1% of requests |
| 99.5% | 216 min/month | 0.5% of requests |
| 99.0% | 432 min/month | 1.0% of requests |

---

## Step 6 — Present the SRE Report

Combine all steps into a structured report:

```
=== SRE Intel — Observability Dashboard ===
Prometheus  : http://prometheus:9090
Alertmanager: http://alertmanager:9093
Range       : 1h | SLO Target: 99.9%
Generated   : 2024-01-15 11:30:00 UTC

--- Firing Alerts ---
Critical: 2  Warning: 5  Total: 7

SEVERITY   STATE    ALERT                 SINCE
critical   firing   HighErrorRate         2024-01-15 10:23
critical   firing   PodCrashLooping       2024-01-15 09:45
warning    firing   HighMemoryUsage       2024-01-15 11:01

--- Service SLIs & SLOs ---
Healthy: 8  Degraded: 2  Total: 10

SERVICE              ERR%    REQ/S  P50ms  P95ms  P99ms  AVAIL(30d)  BUDGET  BURN
payment-service      5.23    142.3  12     89     234    99.23%      77.0%   healthy
user-service         0.12    892.1  8      45     123    99.91%      90.0%   healthy
checkout-service     12.41   234.5  45     312    891    98.12%      0.0%    exhausted 🔴
...
```

---

## Additional PromQL Queries

```bash
# Apdex score (satisfied + tolerating/2 / total)
# satisfied: < 0.1s, tolerating: < 0.4s
curl -sG "$PROM/api/v1/query" \
  --data-urlencode "query=(sum(rate(http_request_duration_seconds_bucket{le=\"0.1\"}[1h])) by (job) + sum(rate(http_request_duration_seconds_bucket{le=\"0.4\"}[1h])) by (job)) / 2 / sum(rate(http_request_duration_seconds_count[1h])) by (job)"

# Top 5 services by error rate
curl -sG "$PROM/api/v1/query" \
  --data-urlencode "query=topk(5, 100 * sum(rate(http_requests_total{status=~\"5..\"}[1h])) by (job) / sum(rate(http_requests_total[1h])) by (job))"

# Alert firing duration (minutes)
curl -sG "$PROM/api/v1/query" \
  --data-urlencode "query=(time() - ALERTS{alertstate=\"firing\"}) / 60"
```

---

## Filtering by Service

To filter results to a specific service, add a label selector to the PromQL queries:

```bash
# Error rate for payment-service only
curl -sG "$PROM/api/v1/query" \
  --data-urlencode "query=100 * sum(rate(http_requests_total{status=~\"5..\",job=~\".*payment.*\"}[1h])) / sum(rate(http_requests_total{job=~\".*payment.*\"}[1h]))"
```

---

## References

- [Prometheus HTTP API](https://prometheus.io/docs/prometheus/latest/querying/api/)
- [Alertmanager API](https://github.com/prometheus/alertmanager/blob/main/api/v2/openapi.yaml)
- [Google SRE Book: SLOs](https://sre.google/sre-book/service-level-objectives/)
- [OpenSLO](https://github.com/OpenSLO/OpenSLO)
