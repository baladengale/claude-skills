---
name: market-overview
description: World market overview with global indices, commodities, crypto, forex, portfolio tracking, stock details, dividends, earnings, and financials.
metadata:
  emoji: "🌍"
  requires:
    bins: ["curl", "bash"]
---

# Market Overview — World Markets & Portfolio Intelligence

Plain-language guide for generating a world market overview. Claude retrieves data from Yahoo Finance public APIs and formats it into structured tables covering global indices, commodities, crypto, currencies, and portfolios.

## When to Activate

Activate when the user asks about:
- World markets, market overview, market summary
- Stock prices, portfolio view, my stocks
- Stock detail, stock analysis
- Dividends, earnings, financials, cashflow
- Top movers, gainers, losers

---

## How to Generate a Market Overview

### Step 1 — Retrieve Live Quotes from Yahoo Finance

Yahoo Finance provides a free batch quote endpoint. Fetch all symbols in one call:

```bash
# Batch quote endpoint — comma-separate up to ~100 symbols
curl -s "https://query2.finance.yahoo.com/v7/finance/quote?symbols=^GSPC,^DJI,^IXIC,^RUT,^VIX,^FTSE,^GDAXI,^FCHI,^STOXX50E,^N225,^HSI,000001.SS,^STI,^BSESN,^NSEI,^KS11,^TWII,GC=F,SI=F,CL=F,BZ=F,BTC-USD,ETH-USD,EURUSD=X,GBPUSD=X,USDJPY=X,USDCNY=X,USDINR=X,USDSGD=X,USDMYR=X" \
  -H "User-Agent: Mozilla/5.0" | jq '.quoteResponse.result[] | {symbol, shortName, regularMarketPrice, regularMarketChangePercent}'
```

**Key fields to extract per symbol:**
- `regularMarketPrice` — current price
- `regularMarketChangePercent` — % change today
- `regularMarketChange` — absolute change
- `regularMarketPreviousClose` — yesterday's close
- `currency` — quote currency

### Step 2 — Format the Output Tables

Present as a series of tables. Use ▲ for positive change, ▼ for negative.

**Template:**

```
=== WORLD MARKETS — {date} ===

US MARKETS
NAME          PRICE      CHG%
S&P 500       5,234.18   ▲ +0.82%
Dow Jones     39,142.23  ▲ +0.54%
NASDAQ        16,742.39  ▲ +1.11%
Russell 2000  2,082.14   ▼ -0.23%
VIX           14.32      ▼ -3.21%

EUROPEAN MARKETS
NAME          PRICE      CHG%
FTSE 100      8,312.45   ▲ +0.31%
DAX           18,501.23  ▲ +0.67%
CAC 40        8,023.11   ▼ -0.12%
Euro Stoxx 50 4,982.34   ▲ +0.44%

ASIAN MARKETS
NAME          PRICE      CHG%
Nikkei 225    38,892.10  ▲ +1.23%
Hang Seng     16,512.34  ▼ -0.88%
Shanghai      3,041.23   ▲ +0.22%
STI Singapore 3,211.45   ▲ +0.11%
Sensex        72,341.23  ▲ +0.54%
Nifty 50      21,892.34  ▲ +0.61%
KOSPI         2,631.45   ▼ -0.33%
TAIEX         18,923.11  ▲ +0.89%

COMMODITIES & CRYPTO
NAME          PRICE        CHG%
Gold          2,312.40     ▲ +0.43%
Silver        27.32        ▲ +0.22%
Crude Oil WTI 81.23        ▼ -0.54%
Brent Crude   85.11        ▼ -0.41%
Bitcoin       67,234.00    ▲ +2.11%
Ethereum      3,412.00     ▲ +1.88%

CURRENCIES (USD base)
PAIR       RATE    CHG%
USD/EUR    0.9234  ▲ +0.12%
USD/GBP    0.7891  ▲ +0.08%
USD/JPY    151.32  ▼ -0.23%
USD/CNY    7.2341  ▼ -0.11%
USD/INR    83.41   ▼ -0.09%
USD/SGD    1.3421  ▼ -0.14%
USD/MYR    4.7123  ▼ -0.31%
SGD/INR    62.14   ▲ +0.05%
SGD/MYR    3.5112  ▼ -0.17%
```

---

## Tracked Symbols Reference

### All Symbols

| Category | Symbol | Name |
|----------|--------|------|
| US | `^GSPC` | S&P 500 |
| US | `^DJI` | Dow Jones |
| US | `^IXIC` | NASDAQ |
| US | `^RUT` | Russell 2000 |
| US | `^VIX` | VIX |
| Europe | `^FTSE` | FTSE 100 |
| Europe | `^GDAXI` | DAX |
| Europe | `^FCHI` | CAC 40 |
| Europe | `^STOXX50E` | Euro Stoxx 50 |
| Asia | `^N225` | Nikkei 225 |
| Asia | `^HSI` | Hang Seng |
| Asia | `000001.SS` | Shanghai |
| Asia | `^STI` | STI Singapore |
| Asia | `^BSESN` | Sensex |
| Asia | `^NSEI` | Nifty 50 |
| Asia | `^KS11` | KOSPI |
| Asia | `^TWII` | TAIEX |
| Commodities | `GC=F` | Gold |
| Commodities | `SI=F` | Silver |
| Commodities | `CL=F` | Crude Oil WTI |
| Commodities | `BZ=F` | Brent Crude |
| Crypto | `BTC-USD` | Bitcoin |
| Crypto | `ETH-USD` | Ethereum |
| FX | `EURUSD=X` | USD/EUR |
| FX | `GBPUSD=X` | USD/GBP |
| FX | `USDJPY=X` | USD/JPY |
| FX | `USDCNY=X` | USD/CNY |
| FX | `USDINR=X` | USD/INR |
| FX | `USDSGD=X` | USD/SGD |
| FX | `USDMYR=X` | USD/MYR |

### Portfolio Symbols

| Symbol | Name |
|--------|------|
| `TSLA` | Tesla |
| `NVDA` | NVIDIA |
| `V` | Visa |
| `MSFT` | Microsoft |
| `META` | Meta |
| `GOOGL` | Google |
| `AMZN` | Amazon |
| `AMD` | AMD |
| `AVGO` | Broadcom |
| `AAPL` | Apple |

### Top Movers Watchlist

**US:** TSLA, NVDA, AAPL, MSFT, GOOGL, AMZN, META, AMD, AVGO, V, JPM, BRK-B, UNH, XOM, JNJ, WMT

**Singapore:** D05.SI (DBS), O39.SI (OCBC), U11.SI (UOB), Z74.SI (Singtel), C6L.SI (SIA), C38U.SI (CapitaLand), G13.SI (Genting SG), S58.SI (SATS)

**India:** RELIANCE.NS, TCS.NS, INFY.NS, HDFCBANK.NS, ICICIBANK.NS, HINDUNILVR.NS, ITC.NS, SBIN.NS, BHARTIARTL.NS, WIPRO.NS

---

## Stock Detail View

For a specific stock (e.g., NVDA), fetch the quote summary:

```bash
# Quote summary with fundamentals (assetProfile, financialData, defaultKeyStatistics)
curl -s "https://query2.finance.yahoo.com/v10/finance/quoteSummary/NVDA?modules=assetProfile,financialData,defaultKeyStatistics,summaryDetail,earnings" \
  -H "User-Agent: Mozilla/5.0" | jq '.quoteSummary.result[0]'
```

**Present stock detail in this format:**
```
=== NVDA — NVIDIA Corporation ===
Price:    $875.40   ▲ +2.34% today
52w High: $974.00   52w Low: $402.10
P/E:      65.2      EPS: $13.42
Market Cap: $2.15T
Revenue (TTM): $60.9B   Net Income: $29.8B
Gross Margin: 74.6%     ROE: 91.2%
```

---

## Dividends

```bash
# Historical dividends
curl -s "https://query2.finance.yahoo.com/v8/finance/chart/AAPL?events=dividends&range=2y&interval=3mo" \
  -H "User-Agent: Mozilla/5.0" | jq '.chart.result[0].events.dividends'
```

Format output:
```
=== AAPL Dividend History ===
Date        Amount   Yield
2024-02-09  $0.24    0.55%
2023-11-10  $0.24    0.53%
...
```

---

## Earnings

```bash
# Earnings data
curl -s "https://query2.finance.yahoo.com/v10/finance/quoteSummary/TSLA?modules=earningsHistory,earningsTrend" \
  -H "User-Agent: Mozilla/5.0" | jq '.quoteSummary.result[0].earningsHistory'
```

Format output:
```
=== TSLA Earnings History ===
Quarter   EPS Actual  EPS Est   Surprise
Q3 2024   $0.72       $0.58     +24.1%
Q2 2024   $0.52       $0.61     -14.8%
...
```

---

## Financials

```bash
# Income statement (yearly)
curl -s "https://query2.finance.yahoo.com/v10/finance/quoteSummary/NVDA?modules=incomeStatementHistory" \
  -H "User-Agent: Mozilla/5.0" | jq '.quoteSummary.result[0].incomeStatementHistory'

# Income statement (quarterly)
curl -s "https://query2.finance.yahoo.com/v10/finance/quoteSummary/NVDA?modules=incomeStatementHistoryQuarterly" \
  -H "User-Agent: Mozilla/5.0" | jq '.quoteSummary.result[0].incomeStatementHistoryQuarterly'
```

---

## Cashflow

```bash
# Cash flow statement
curl -s "https://query2.finance.yahoo.com/v10/finance/quoteSummary/MSFT?modules=cashflowStatementHistory" \
  -H "User-Agent: Mozilla/5.0" | jq '.quoteSummary.result[0].cashflowStatementHistory'
```

---

## Interpreting Market Data

**Indices:** A positive S&P 500 and NASDAQ with VIX below 20 indicates risk-on sentiment.

**Commodities:** Gold rising with equities falling = flight to safety. Oil rising = inflation pressure.

**Crypto:** BTC/ETH moves often precede or reflect broad risk appetite.

**Currencies:** USD strengthening (USD/JPY rising) = risk-off globally; USD/EUR falling = USD weakening.

**Cross-rates (SGD/INR, SGD/MYR):** Calculated as USDINR/USDSGD and USDMYR/USDSGD respectively.

---

## References

- [Yahoo Finance](https://finance.yahoo.com/)
- [Yahoo Finance API (unofficial)](https://github.com/ranaroussi/yfinance)
