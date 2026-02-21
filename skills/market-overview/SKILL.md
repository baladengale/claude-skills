---
name: market-overview
description: World market overview with HTML email and terminal output - global indices, commodities, crypto, forex, portfolio tracking, stock details, dividends, earnings, and financials.
metadata:
  emoji: "🌍"
  requires:
    bins: ["go"]
---

# Market Overview (Go) - World Markets & Portfolio Tracker

Go-based market overview tool with concurrent Yahoo Finance data fetching, ANSI terminal tables, and responsive HTML email output.

## When to Activate

Activate when the user asks about:
- World markets, market overview, market summary
- Stock prices, portfolio view, my stocks
- Stock detail, stock analysis
- Dividends, earnings, financials, cashflow
- Top movers, gainers, losers

## Script Location

```
skills/market-overview/main.go
```

## Usage

### Default Detail View (all world markets)
```bash
./market-overview
```

### Summary View (key indices only)
```bash
./market-overview -s
```

### Portfolio View (personal holdings)
```bash
./market-overview -p
```

### Stock Detail
```bash
./market-overview -t NVDA
```

### Dividends
```bash
./market-overview -d AAPL
```

### Earnings
```bash
./market-overview -e TSLA
```

### Financials (yearly / quarterly)
```bash
./market-overview -f NVDA
./market-overview -f NVDA -q
```

### Cashflow (yearly / quarterly)
```bash
./market-overview -c MSFT
./market-overview -c MSFT -q
```

### Skip movers section
```bash
./market-overview --no-movers
```

### Email control
```bash
./market-overview --no-email      # terminal only
./market-overview --email-only    # email only, no terminal output
```

## Build

```bash
cd skills/market-overview
make build          # compile for current platform
make run            # build and run
make linux          # cross-compile for Linux amd64
make darwin         # cross-compile for macOS arm64
make install        # build and copy to ~/.claude-skills/skills/market-overview/
```

## Environment Variables

Loaded from `.env` in the current directory:

- `GMAIL_USER` — Gmail address
- `GMAIL_APP_PASSWORD` — Gmail app password (required for email)
- `MARKET_RECIPIENTS` — Comma-separated recipient list

## Data Sources

All data fetched from Yahoo Finance via direct HTTP API:
- Batch quotes (v7 endpoint) for current prices
- Chart data (v8 endpoint) for historical changes
- Quote summary (v10 endpoint) for fundamentals

## Markets Tracked

**Indices:** S&P 500, Dow Jones, NASDAQ, Russell 2000, VIX, FTSE 100, DAX, CAC 40, Euro Stoxx 50, Nikkei 225, Hang Seng, Shanghai, STI, Sensex, Nifty 50, KOSPI, TAIEX

**Commodities & Crypto:** Gold, Silver, Crude Oil WTI, Brent Crude, Bitcoin, Ethereum

**Currencies:** USD/EUR, USD/GBP, USD/JPY, USD/CNY, USD/INR, USD/SGD, USD/MYR, SGD/INR, SGD/MYR

**Portfolio:** Tesla, NVIDIA, Visa, Microsoft, Meta, Google, Amazon, AMD, Broadcom, Apple

**Movers:** 15 US + 8 Singapore + 10 India stocks
