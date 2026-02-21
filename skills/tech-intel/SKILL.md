---
name: tech-intel
description: Daily Market & Tech Intelligence newsletter - concurrent RSS aggregation, keyword scoring, and email delivery via Gmail SMTP.
metadata:
  emoji: "📡"
  requires:
    bins: ["go"]
---

# Tech Intel - Daily Market & Tech Pulse

Concurrent RSS feed aggregator that scores articles by keyword relevance and delivers a curated top-10 HTML newsletter via email.

## When to Activate

Activate when the user asks about:
- Tech news, tech intelligence, market intelligence
- Daily newsletter, tech newsletter, market newsletter
- Send tech digest, send market digest
- RSS feeds, news aggregation
- Tech pulse, market pulse

## Script Location

```
skills/tech-intel/main.go
```

## Usage

### Build the binary
```bash
cd skills/tech-intel && make build
```

### Run the newsletter (fetch + score + email)
```bash
skills/tech-intel/tech-intel
```

### Build and run in one step
```bash
cd skills/tech-intel && make run
```

### Run from source (no build step)
```bash
go run skills/tech-intel/main.go
```

## What It Does

1. Fetches 12 RSS feeds concurrently using goroutines
2. Filters articles to last 24 hours
3. Deduplicates by URL
4. Scores articles by keyword relevance (AI, Semiconductor, Acquisition, FinTech, etc.)
5. Selects top 10 articles sorted by score and recency
6. Renders a styled HTML newsletter from template
7. Sends via Gmail SMTP to configured recipients
8. Falls back to saving HTML file if email delivery fails

## Environment Variables

Loaded from `.env` in the current directory:

- `GMAIL_USER` - Gmail address
- `GMAIL_APP_PASSWORD` - Gmail app password (required for email delivery)
- `NEWSLETTER_RECIPIENTS` - Comma-separated recipient list

## RSS Sources

TechCrunch, Ars Technica, The Verge, Hacker News, Wired, CNBC (Top + World), MarketWatch, Yahoo Finance, Reuters, BBC Business, NPR Business

## Keyword Scoring

Articles are scored based on keyword matches in title and description:
- **High (8-10):** Market Disruption, Revolutionary, Acquisition, Semiconductor
- **Medium (5-7):** AI, FinTech, IPO, Merger, Regulation, Cybersecurity, Quantum, Data Breach
- **Standard (4):** Startup, Funding, Blockchain

Title matches receive double weight. Recent articles (< 6 hours) get a recency bonus (+3). Markets category articles get +2.

## CLI Reference

```
./tech-intel
```

No flags needed. Configuration is via environment variables.
