# Daily Digest

Unified daily email combining Fundamental Trend Analysis, Price Performance, India MF NAVs, Market Overview, and Tech Intel into one email.

## Features

- **🧠 Fundamental Trend Analysis** — 24 instruments (US stocks, ETFs, Singapore, India) scored via Piotroski F-Score + technical indicators. Composite = Fundamental×60% + Technical×40%
- **📈 Price Performance** — 1W / 1M / 3M / 1Y % change for all FTA tickers, grouped by region
- **🇮🇳 India Mutual Funds** — Live NAV via [mfapi.in](https://api.mfapi.in) with 1W/1M/3M/1Y % change
- **📈 Market Overview** — Global indices, commodities, crypto, forex, US/SG movers
- **📰 Tech Intel** — Top scored news from RSS feeds (TechCrunch, Wired, Reuters, etc.)

## Tickers Covered

| Region | Tickers |
|--------|---------|
| 🇺🇸 US Stocks | V, NVDA, NFLX, MSFT, AAPL, GOOGL, META, AVGO, AMD, PLTR |
| 📊 US ETFs | VOO, QQQ, SPY, SPMO, IBIT |
| 🇸🇬 Singapore | Z74.SI (Singtel), C6L.SI (SIA), HST.SI (Lion HSTECH), D05.SI (DBS) |
| 🇮🇳 India | ITC.NS, RELIANCE.NS, TMCV.NS (Tata Motors), ^NSEI (NIFTY 50), GC=F (Gold/SGB) |

## India MF Scheme Codes

| Fund | Scheme Code |
|------|-------------|
| Parag Parikh Flexi Cap (Direct Growth) | 122639 |
| Mirae Asset Large & Midcap (Direct Growth) | 118834 |
| Axis Midcap (Direct Growth) | 120505 |
| Axis Small Cap (Direct Growth) | 125354 |

## Setup

```bash
pip install yfinance feedparser requests
```

Configure Gmail credentials and recipients inside the script (search for `GMAIL_USER`).

## Cron

Runs daily at `5 23 * * *` UTC (7:05 AM SGT) via OpenClaw cron.

## TODO
- Add unit holdings to `INDIA_MF_SCHEMES` for portfolio value tracking
- Integrate Kuvera API for direct portfolio sync
