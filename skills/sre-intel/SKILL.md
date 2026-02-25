---
name: sre-intel
description: SRE observability intelligence - queries Prometheus for HTTP error rates and latency SLIs, Alertmanager for firing alerts, and computes 30-day SLO compliance with error budget burn rates.
metadata:
  emoji: "📊"
  requires:
    bins: ["go"]
---

# SRE Intel — Prometheus & Alertmanager Intelligence

Prometheus and Alertmanager intelligence tool for SRE teams. Queries HTTP APIs directly (no external libraries) to surface error rates, latency percentiles, SLO compliance, firing alerts, and error budget burn.

Inspired by open-source SRE tooling patterns (Google SRE Book, OpenSLO, sloth, pyrra).

## When to Activate

Activate when the user asks about:
- SRE dashboard, SLO status, error budget, SLI
- Prometheus metrics, alert status, alertmanager
- Service error rates, latency p99, p95, p50
- Firing alerts, alert summary, on-call intelligence
- Error budget burn, SLO compliance, availability
- Incident overview, service health scorecard

## Script Location

```
skills/sre-intel/main.go
```

## Usage

### Build the binary
```bash
cd skills/sre-intel && make build
```

### Run with Prometheus URL
```bash
PROMETHEUS_URL=http://prometheus:9090 skills/sre-intel/sre-intel
```

### Filter by service/job
```bash
skills/sre-intel/sre-intel -s payment-service
```

### Custom time range
```bash
skills/sre-intel/sre-intel -r 6h
```

### Send email report
```bash
skills/sre-intel/sre-intel -email
```

### Save HTML report
```bash
skills/sre-intel/sre-intel -html sre-report.html
```

## What It Monitors

1. **Firing Alerts** — All active alerts from Alertmanager by severity (critical → warning → info)
2. **Error Rates** — HTTP 5xx error % per service over configurable window (default: 1h)
3. **Latency SLIs** — P50 / P95 / P99 response times per service
4. **SLO Compliance** — 30-day availability % against 99.9% SLO target
5. **Error Budget** — Remaining error budget in minutes, burn rate classification

## SLO Defaults

| SLO Target | Error Budget (30d) | Allowed Errors |
|------------|-------------------|----------------|
| 99.9% | 43.2 min/month | 0.1% of requests |
| 99.5% | 216 min/month | 0.5% of requests |
| 99.0% | 432 min/month | 1.0% of requests |

## Environment Variables

Loaded from `.env` in the current directory:

- `PROMETHEUS_URL` — Prometheus base URL (e.g. `http://prometheus:9090`)
- `ALERTMANAGER_URL` — Alertmanager base URL (e.g. `http://alertmanager:9093`)
- `GMAIL_USER` — Gmail address (for email)
- `GMAIL_APP_PASSWORD` — Gmail app password (for email)
- `SRE_RECIPIENTS` — Comma-separated recipient list

## CLI Reference

```
-u string    Prometheus URL (overrides PROMETHEUS_URL env)
-am string   Alertmanager URL (overrides ALERTMANAGER_URL env)
-s string    Filter by service/job name (substring match)
-r string    Lookback range for error/latency metrics (default: 1h)
-slo float   SLO target percentage (default: 99.9)
-email       Send email report
-html file   Save HTML report to file
```
