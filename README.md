# Claude Skills — Market Intelligence Tools

Two Go-based skills for Claude Code that deliver financial market data and curated tech news via terminal output and HTML email.

---

## Skills Overview

| Skill | Binary | Purpose |
|-------|--------|---------|
| [market-overview](skills/market-overview/) | `market-overview` | World markets, stocks, portfolio, dividends, earnings, financials |
| [tech-intel](skills/tech-intel/) | `tech-intel` | Daily tech + market news newsletter from 12 RSS feeds |

Both are self-contained Go programs with **zero external dependencies** — only the Go standard library is used.

---

## Platform Requirements

> **Use Ubuntu (Linux) for building and running these tools.**

The existing binaries checked into this repo are **Linux ELF amd64 executables** and will not run on Windows. The Makefiles use Unix shell syntax (`GOOS=`, `rm -f`, `./binary`).

| Requirement | Linux/macOS | Windows |
|-------------|-------------|---------|
| Pre-built binaries | Run directly | Will not work (ELF format) |
| `make build` | Works natively | Needs Git Bash + Go + GNU Make |
| ANSI colors in terminal | Native | Works in Windows Terminal only |
| `~/.claude-skills/.env` config | Native `$HOME` | May need manual path setup |

**Recommendation:** Clone this repo on your Ubuntu box, build there, test there.

---

## Setup (Ubuntu / Linux)

### 1. Install Go

```bash
# Ubuntu 22.04+
sudo apt install golang-go

# Or install a specific version
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

### 2. Configure Environment

Create a `.env` file in the directory where you run the binary (next to the binary or in your working directory):

```bash
cat > .env << 'EOF'
GMAIL_USER=you@gmail.com
GMAIL_APP_PASSWORD=xxxx-xxxx-xxxx-xxxx
MARKET_RECIPIENTS=you@gmail.com,colleague@example.com
NEWSLETTER_RECIPIENTS=you@gmail.com,colleague@example.com
EOF
chmod 600 .env
```

> **Gmail App Password:** Go to Google Account → Security → 2-Step Verification → App Passwords. Generate one for "Mail".

> **Note:** The `.env` file is loaded from the current working directory. Works the same on any machine with no special directory names required.

### 3. Clone and Build

```bash
git clone <your-repo-url> claude-skills
cd claude-skills

# Build market-overview
cd skills/market-overview && make build

# Build tech-intel
cd ../tech-intel && make build
```

---

## Skill 1: Market Overview

**Location:** [skills/market-overview/](skills/market-overview/)

Fetches live data from Yahoo Finance and renders ANSI-colored tables in the terminal plus an HTML email report. All HTTP calls are concurrent (up to 10 goroutines).

### What It Tracks

- **US Markets:** S&P 500, Dow Jones, NASDAQ, Russell 2000, VIX
- **European Markets:** FTSE 100, DAX, CAC 40, Euro Stoxx 50
- **Asian Markets:** Nikkei 225, Hang Seng, Shanghai, STI, Sensex, Nifty 50, KOSPI, TAIEX
- **Commodities & Crypto:** Gold, Silver, Crude Oil WTI, Brent Crude, Bitcoin, Ethereum
- **Currencies:** USD/EUR, USD/GBP, USD/JPY, USD/CNY, USD/INR, USD/SGD, USD/MYR, SGD/INR, SGD/MYR
- **Portfolio:** Tesla, NVIDIA, Visa, Microsoft, Meta, Google, Amazon, AMD, Broadcom, Apple
- **Top Movers:** 15 US + 8 Singapore + 10 India stocks

### Historical Periods Shown

Each market shows 1D change plus historical performance for: 1W, 1M, 3M, 6M, 1Y, 2Y, 5Y

### Usage

```bash
cd skills/market-overview

# Default: full world markets overview (terminal + email)
./market-overview

# Summary view — key indices only
./market-overview -s

# Portfolio view — your stocks only
./market-overview -p

# Stock detail for a ticker
./market-overview -t NVDA

# Dividend history
./market-overview -d AAPL

# Earnings dates and EPS history
./market-overview -e TSLA

# Income statement (yearly by default)
./market-overview -f NVDA
./market-overview -f NVDA -q      # quarterly

# Cashflow statement
./market-overview -c MSFT
./market-overview -c MSFT -q      # quarterly

# Skip top movers section
./market-overview --no-movers

# Terminal only (no email)
./market-overview --no-email

# Email only (no terminal output)
./market-overview --email-only
```

### Build

```bash
cd skills/market-overview

make build          # compile for current platform
make run            # build and run immediately
make linux          # cross-compile → market-overview-linux-amd64
make darwin         # cross-compile → market-overview-darwin-arm64
make all            # both linux and darwin
make install        # build + copy to ~/.claude-skills/workspace/skills/market-overview/
make clean          # remove build artifacts
make fmt            # gofmt the source
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `GMAIL_USER` | Gmail address (sender) |
| `GMAIL_APP_PASSWORD` | Gmail App Password (required for email) |
| `MARKET_RECIPIENTS` | Comma-separated recipient list |

Email is skipped silently if `GMAIL_APP_PASSWORD` is not set. Terminal output still works.

### Data Sources

All data is fetched from Yahoo Finance:
- **Batch quotes** — `v7/finance/quote` endpoint
- **Historical chart data** — `v8/finance/chart` endpoint (10-year range, 1d interval)
- **Fundamentals** — `v10/finance/quoteSummary` endpoint

Yahoo Finance auth (cookie + crumb) is initialized automatically at startup. Requests use retry logic with backoff (3 attempts, 2s delay).

---

## Skill 2: Tech Intel

**Location:** [skills/tech-intel/](skills/tech-intel/)

Aggregates 12 RSS feeds concurrently, scores articles by keyword relevance, and delivers a curated top-25 HTML newsletter via email. Falls back to saving an HTML file if email is unavailable.

### RSS Sources (12 feeds)

| Source | Category |
|--------|----------|
| TechCrunch | Tech |
| Ars Technica | Tech |
| The Verge | Tech |
| Hacker News (≥100 pts) | Tech |
| Wired | Tech |
| CNBC Top News | Markets |
| CNBC World | Markets |
| MarketWatch | Markets |
| Yahoo Finance | Markets |
| BBC Business | Business |
| NPR Business | Business |
| Reuters | Markets |

### Scoring Algorithm

Articles are scored by keyword matches in title + description. Title matches receive double weight.

| Score | Keywords |
|-------|----------|
| 10 | market disruption |
| 9 | revolutionary |
| 8 | acquisition, semiconductor |
| 7 | fintech, ai, artificial intelligence |
| 6 | ipo, merger, data breach |
| 5 | regulation, earnings, quarterly results, cybersecurity, quantum |
| 4 | startup, funding, blockchain |
| +3 | recency bonus (published < 6 hours ago) |
| +2 | Markets category source |

### Pipeline

1. Fetch 12 feeds concurrently (goroutines, 15s timeout each)
2. Filter to last 24 hours
3. Deduplicate by normalized URL
4. Score all articles
5. Sort by score DESC, then recency DESC
6. Select top 25
7. Render HTML newsletter from embedded template
8. Send via Gmail SMTP — or save to `~/.claude-skills/workspace/tech_intel_YYYYMMDD_HHMMSS.html`

### Usage

```bash
cd skills/tech-intel

# Run the full pipeline (fetch → score → email or save)
./tech-intel

# Build and run in one step
make run

# Run from source without building
go run main.go
```

### Build

```bash
cd skills/tech-intel

make build          # compile for current platform
make run            # build and run immediately
make linux          # cross-compile → tech-intel-linux-amd64
make darwin         # cross-compile → tech-intel-darwin-arm64
make all            # both linux and darwin
make install        # build + copy to ~/.claude-skills/workspace/skills/tech-intel/
make clean          # remove build artifacts
make fmt            # gofmt the source
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `GMAIL_USER` | Gmail address (sender) |
| `GMAIL_APP_PASSWORD` | Gmail App Password (required for email) |
| `NEWSLETTER_RECIPIENTS` | Comma-separated recipient list |

If `GMAIL_APP_PASSWORD` is not set, the rendered HTML is saved to the current directory instead.

---

## Claude Code Integration

These skills are activated by Claude Code when you ask about the topics listed in each `SKILL.md`. Claude will build the binary if needed and run the appropriate command.

### Trigger Phrases

**market-overview:**
- "show me world markets", "market overview", "market summary"
- "my stocks", "portfolio view"
- "stock detail for NVDA", "AAPL dividends", "TSLA earnings"
- "MSFT financials", "NVDA cashflow"

**tech-intel:**
- "send tech digest", "tech newsletter", "market pulse"
- "tech news", "daily newsletter"
- "RSS feeds", "news aggregation"

### SKILL.md Files

Each skill has a `SKILL.md` with YAML front-matter read by Claude Code:

```yaml
---
name: market-overview
description: World market overview...
metadata:
  emoji: "🌍"
    requires:
      bins: ["go"]
---
```

The `requires.bins` field tells Claude Code that `go` must be available to build the skill.

---

## Repository Structure

```
claude-skills/
├── README.md
└── skills/
    ├── market-overview/
    │   ├── SKILL.md          # Claude Code skill descriptor
    │   ├── main.go           # ~1,576 lines — all logic, no dependencies
    │   ├── template.html     # Embedded HTML email template
    │   ├── go.mod            # module github.com/baladengale/claude-skills-market-overview
    │   ├── Makefile          # build / run / cross-compile targets
    │   └── market-overview   # Pre-built Linux amd64 binary
    └── tech-intel/
        ├── SKILL.md          # Claude Code skill descriptor
        ├── main.go           # ~589 lines — all logic, no dependencies
        ├── template.html     # Embedded HTML email template
        ├── go.mod            # module github.com/baladengale/claude-skills-tech-intel
        ├── Makefile          # build / run / cross-compile targets
        └── tech-intel        # Pre-built Linux amd64 binary
```

---

## Quick Start (Ubuntu)

```bash
# 1. Ensure Go is installed
go version   # need 1.22+

# 2. Build both skills
cd skills/market-overview && make build
cd ../tech-intel && make build
cd ../..

# 3. Create .env in each skill directory
cat > skills/market-overview/.env << 'EOF'
GMAIL_USER=you@gmail.com
GMAIL_APP_PASSWORD=your-app-password
MARKET_RECIPIENTS=you@gmail.com
EOF

cat > skills/tech-intel/.env << 'EOF'
GMAIL_USER=you@gmail.com
GMAIL_APP_PASSWORD=your-app-password
NEWSLETTER_RECIPIENTS=you@gmail.com
EOF

# 4. Run market overview (terminal only, must run from skill dir so .env is found)
cd skills/market-overview && ./market-overview --no-email

# 5. Run tech intel (saves HTML to current dir if email not configured)
cd ../tech-intel && ./tech-intel
```
