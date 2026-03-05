---
name: fundamental-trend-analysis
description: Comprehensive stock analysis combining fundamental scoring (Piotroski F-Score, valuation, growth, financial health) with technical trend signals to produce a 6-tier buy/sell rating from Must Buy to Must Sell.
metadata:
  emoji: "📊"
  requires:
    bins: ["curl", "bash", "jq"]
---

# Fundamental Trend Analysis — Composite Buy/Sell Signal Engine

Plain-language guide for generating a comprehensive stock analysis that combines **fundamental scoring** (financial health, valuation, profitability, growth) with **technical trend signals** (moving averages, momentum, price action) to produce a clear **6-tier rating** from Must Buy (✅✅✅) to Must Sell (❌❌❌).

This skill is inspired by proven quantitative frameworks: **Piotroski F-Score**, **Altman Z-Score**, and composite scoring systems used by Danelfin, Tickeron, and institutional quant desks.

## When to Activate

Activate when the user asks about:
- Stock analysis, fundamental analysis, should I buy/sell
- Stock rating, stock score, stock signal
- Fundamental trend, composite score
- Buy signal, sell signal, strong buy, must buy
- Is [TICKER] a good buy, rate this stock
- Analyze [TICKER] fundamentals

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────┐
│              COMPOSITE SIGNAL ENGINE                 │
│                                                     │
│  ┌──────────────────┐    ┌──────────────────────┐   │
│  │  FUNDAMENTAL      │    │  TECHNICAL TREND      │   │
│  │  SCORE (0-100)    │    │  SCORE (0-100)        │   │
│  │                   │    │                       │   │
│  │  Profitability 25 │    │  MA Alignment    25   │   │
│  │  Valuation     25 │    │  Momentum        25   │   │
│  │  Financial Hlth 25│    │  Price Action    25   │   │
│  │  Growth        25 │    │  Volume Trend    25   │   │
│  └────────┬──────────┘    └───────────┬───────────┘   │
│           │                           │               │
│           └─────────┬─────────────────┘               │
│                     ▼                                 │
│           ┌──────────────────┐                        │
│           │ COMPOSITE SCORE  │                        │
│           │ Fundamental × 60%│                        │
│           │ Technical   × 40%│                        │
│           └────────┬─────────┘                        │
│                    ▼                                  │
│  ✅✅✅ Must Buy  ·  ✅✅ Strong Buy  ·  ✅ Buy       │
│  ⚪ Hold  ·  ❌ Sell  ·  ❌❌ Strong Sell             │
│  ❌❌❌ Must Sell                                     │
└─────────────────────────────────────────────────────┘
```

---

## Step 1 — Fetch Fundamental Data from Yahoo Finance

Fetch all fundamental data needed for scoring in a single call:

```bash
# Fetch comprehensive fundamentals for a ticker
TICKER="NVDA"
curl -s "https://query2.finance.yahoo.com/v10/finance/quoteSummary/${TICKER}?modules=financialData,defaultKeyStatistics,summaryDetail,incomeStatementHistory,incomeStatementHistoryQuarterly,balanceSheetHistory,balanceSheetHistoryQuarterly,cashflowStatementHistory,earnings,earningsTrend" \
  -H "User-Agent: Mozilla/5.0" | jq '.quoteSummary.result[0]'
```

**Key fields to extract:**

| Category | Fields | Source Module |
|----------|--------|---------------|
| Profitability | `returnOnEquity`, `returnOnAssets`, `profitMargins`, `operatingCashflow`, `netIncomeToCommon` | `financialData` |
| Valuation | `trailingPE`, `forwardPE`, `priceToBook`, `pegRatio`, `enterpriseToEbitda` | `defaultKeyStatistics`, `summaryDetail` |
| Financial Health | `debtToEquity`, `currentRatio`, `quickRatio`, `totalDebt`, `totalCash` | `financialData` |
| Growth | `revenueGrowth`, `earningsGrowth`, `earningsQuarterlyGrowth` | `financialData`, `earningsTrend` |
| Price Context | `fiftyTwoWeekHigh`, `fiftyTwoWeekLow`, `fiftyDayAverage`, `twoHundredDayAverage` | `summaryDetail` |

---

## Step 2 — Fetch Technical / Price Data

```bash
# Historical price data — 1 year daily for MA/momentum calculations
curl -s "https://query2.finance.yahoo.com/v8/finance/chart/${TICKER}?range=1y&interval=1d" \
  -H "User-Agent: Mozilla/5.0" | jq '{
    closes: .chart.result[0].indicators.quote[0].close,
    volumes: .chart.result[0].indicators.quote[0].volume,
    highs: .chart.result[0].indicators.quote[0].high,
    lows: .chart.result[0].indicators.quote[0].low,
    timestamps: .chart.result[0].timestamp
  }'
```

```bash
# Current quote for latest price and change
curl -s "https://query2.finance.yahoo.com/v7/finance/quote?symbols=${TICKER}" \
  -H "User-Agent: Mozilla/5.0" | jq '.quoteResponse.result[0] | {
    regularMarketPrice,
    regularMarketChangePercent,
    regularMarketVolume,
    averageDailyVolume3Month,
    fiftyDayAverage,
    twoHundredDayAverage,
    fiftyTwoWeekHigh,
    fiftyTwoWeekLow
  }'
```

---

## Step 3 — Calculate Fundamental Score (0–100)

The fundamental score is divided into 4 pillars, each worth 25 points max.

### Pillar 1: Profitability (0–25 points)

Based on the **Piotroski F-Score** profitability criteria, extended with margin quality.

| Metric | Condition | Points |
|--------|-----------|--------|
| Return on Equity (ROE) | > 15% = 7pts, > 10% = 5pts, > 5% = 3pts, > 0% = 1pt | 0–7 |
| Return on Assets (ROA) | > 10% = 6pts, > 5% = 4pts, > 2% = 2pts, > 0% = 1pt | 0–6 |
| Profit Margin | > 20% = 6pts, > 10% = 4pts, > 5% = 2pts, > 0% = 1pt | 0–6 |
| Operating Cash Flow Positive | Yes = 3pts, and OCF > Net Income = +3pts | 0–6 |
| **Pillar Total** | | **0–25** |

**Scoring logic:**
```
profitability_score = 0

# ROE scoring
if roe > 0.15: profitability_score += 7
elif roe > 0.10: profitability_score += 5
elif roe > 0.05: profitability_score += 3
elif roe > 0: profitability_score += 1

# ROA scoring
if roa > 0.10: profitability_score += 6
elif roa > 0.05: profitability_score += 4
elif roa > 0.02: profitability_score += 2
elif roa > 0: profitability_score += 1

# Profit margin scoring
if profit_margin > 0.20: profitability_score += 6
elif profit_margin > 0.10: profitability_score += 4
elif profit_margin > 0.05: profitability_score += 2
elif profit_margin > 0: profitability_score += 1

# Cash flow quality
if operating_cashflow > 0: profitability_score += 3
if operating_cashflow > net_income: profitability_score += 3

profitability_score = min(profitability_score, 25)
```

### Pillar 2: Valuation (0–25 points)

Measures whether the stock is fairly priced relative to its earnings, book value, and growth.

| Metric | Condition | Points |
|--------|-----------|--------|
| Trailing P/E | < 15 = 7pts, < 20 = 5pts, < 30 = 3pts, < 50 = 1pt, ≥ 50 or negative = 0 | 0–7 |
| Forward P/E | < Trailing P/E (improving) = 4pts, within 10% = 2pts | 0–4 |
| Price-to-Book | < 1.5 = 5pts, < 3 = 3pts, < 5 = 2pts, < 10 = 1pt | 0–5 |
| PEG Ratio | < 1.0 = 5pts, < 1.5 = 3pts, < 2.0 = 2pts, < 3.0 = 1pt | 0–5 |
| EV/EBITDA | < 10 = 4pts, < 15 = 3pts, < 20 = 2pts, < 30 = 1pt | 0–4 |
| **Pillar Total** | | **0–25** |

**Scoring logic:**
```
valuation_score = 0

# Trailing P/E (lower is better for value)
if 0 < trailing_pe < 15: valuation_score += 7
elif trailing_pe < 20: valuation_score += 5
elif trailing_pe < 30: valuation_score += 3
elif trailing_pe < 50: valuation_score += 1

# Forward P/E improvement (earnings expected to grow)
if forward_pe > 0 and trailing_pe > 0:
    if forward_pe < trailing_pe * 0.90: valuation_score += 4
    elif forward_pe < trailing_pe: valuation_score += 2

# Price-to-Book
if 0 < price_to_book < 1.5: valuation_score += 5
elif price_to_book < 3: valuation_score += 3
elif price_to_book < 5: valuation_score += 2
elif price_to_book < 10: valuation_score += 1

# PEG Ratio (growth-adjusted P/E)
if 0 < peg < 1.0: valuation_score += 5
elif peg < 1.5: valuation_score += 3
elif peg < 2.0: valuation_score += 2
elif peg < 3.0: valuation_score += 1

# EV/EBITDA
if 0 < ev_ebitda < 10: valuation_score += 4
elif ev_ebitda < 15: valuation_score += 3
elif ev_ebitda < 20: valuation_score += 2
elif ev_ebitda < 30: valuation_score += 1

valuation_score = min(valuation_score, 25)
```

### Pillar 3: Financial Health (0–25 points)

Measures balance sheet strength, leverage risk, and solvency. Incorporates **Altman Z-Score** concepts.

| Metric | Condition | Points |
|--------|-----------|--------|
| Debt-to-Equity | < 0.3 = 7pts, < 0.5 = 5pts, < 1.0 = 3pts, < 2.0 = 1pt | 0–7 |
| Current Ratio | > 2.0 = 6pts, > 1.5 = 4pts, > 1.0 = 2pts | 0–6 |
| Cash-to-Debt Ratio | cash/debt > 1.0 = 6pts, > 0.5 = 4pts, > 0.25 = 2pts, > 0 = 1pt | 0–6 |
| Interest Coverage* | > 10x = 6pts, > 5x = 4pts, > 2x = 2pts, > 1x = 1pt | 0–6 |
| **Pillar Total** | | **0–25** |

*Interest Coverage = EBIT / Interest Expense (from income statement)*

**Scoring logic:**
```
health_score = 0

# Debt-to-Equity (lower is safer — Buffett prefers < 0.5)
if debt_to_equity < 30: health_score += 7      # Yahoo returns as percentage
elif debt_to_equity < 50: health_score += 5
elif debt_to_equity < 100: health_score += 3
elif debt_to_equity < 200: health_score += 1

# Current Ratio (ability to pay short-term obligations)
if current_ratio > 2.0: health_score += 6
elif current_ratio > 1.5: health_score += 4
elif current_ratio > 1.0: health_score += 2

# Cash-to-Debt Ratio
if total_debt > 0:
    cash_ratio = total_cash / total_debt
    if cash_ratio > 1.0: health_score += 6
    elif cash_ratio > 0.5: health_score += 4
    elif cash_ratio > 0.25: health_score += 2
    elif cash_ratio > 0: health_score += 1
else:
    health_score += 6  # no debt = perfect

# Interest Coverage (from income statement if available)
if interest_expense > 0:
    coverage = ebit / interest_expense
    if coverage > 10: health_score += 6
    elif coverage > 5: health_score += 4
    elif coverage > 2: health_score += 2
    elif coverage > 1: health_score += 1
else:
    health_score += 4  # no interest expense (assume manageable)

health_score = min(health_score, 25)
```

### Pillar 4: Growth (0–25 points)

Measures revenue trajectory, earnings momentum, and forward growth expectations.

| Metric | Condition | Points |
|--------|-----------|--------|
| Revenue Growth (YoY) | > 25% = 7pts, > 15% = 5pts, > 5% = 3pts, > 0% = 1pt | 0–7 |
| Earnings Growth (YoY) | > 25% = 6pts, > 15% = 4pts, > 5% = 2pts, > 0% = 1pt | 0–6 |
| Quarterly Earnings Growth | > 20% = 6pts, > 10% = 4pts, > 0% = 2pt, negative = 0 | 0–6 |
| Analyst Growth Est (Next 5Y) | > 20% = 6pts, > 15% = 4pts, > 10% = 3pts, > 5% = 2pts, > 0% = 1pt | 0–6 |
| **Pillar Total** | | **0–25** |

**Scoring logic:**
```
growth_score = 0

# Revenue Growth YoY
if revenue_growth > 0.25: growth_score += 7
elif revenue_growth > 0.15: growth_score += 5
elif revenue_growth > 0.05: growth_score += 3
elif revenue_growth > 0: growth_score += 1

# Earnings Growth YoY
if earnings_growth > 0.25: growth_score += 6
elif earnings_growth > 0.15: growth_score += 4
elif earnings_growth > 0.05: growth_score += 2
elif earnings_growth > 0: growth_score += 1

# Quarterly Earnings Growth
if quarterly_earnings_growth > 0.20: growth_score += 6
elif quarterly_earnings_growth > 0.10: growth_score += 4
elif quarterly_earnings_growth > 0: growth_score += 2

# Analyst Forward Estimates (earningsTrend: +5y growth)
if analyst_5y_growth > 0.20: growth_score += 6
elif analyst_5y_growth > 0.15: growth_score += 4
elif analyst_5y_growth > 0.10: growth_score += 3
elif analyst_5y_growth > 0.05: growth_score += 2
elif analyst_5y_growth > 0: growth_score += 1

growth_score = min(growth_score, 25)
```

### Total Fundamental Score

```
FUNDAMENTAL_SCORE = profitability_score + valuation_score + health_score + growth_score
# Range: 0–100
```

---

## Step 4 — Calculate Technical Trend Score (0–100)

The technical score uses price history to assess trend strength and momentum.

### Pillar A: Moving Average Alignment (0–25 points)

| Signal | Condition | Points |
|--------|-----------|--------|
| Price > 50-day SMA | Yes | +5 |
| Price > 200-day SMA | Yes | +5 |
| 50-day SMA > 200-day SMA | Yes (Golden Cross territory) | +7 |
| Price > 20-day SMA | Yes (short-term trend up) | +4 |
| 50-day SMA slope positive | Rising over last 20 days | +4 |
| **Pillar Total** | | **0–25** |

**Calculation:**
```
ma_score = 0
price = current_price
sma_20 = average(closes[-20:])
sma_50 = average(closes[-50:])
sma_200 = average(closes[-200:])
sma_50_prev = average(closes[-70:-20])  # 50-day SMA from 20 days ago

if price > sma_50: ma_score += 5
if price > sma_200: ma_score += 5
if sma_50 > sma_200: ma_score += 7      # Golden Cross territory
if price > sma_20: ma_score += 4
if sma_50 > sma_50_prev: ma_score += 4   # 50-day trending up

ma_score = min(ma_score, 25)
```

### Pillar B: Momentum (0–25 points)

| Signal | Condition | Points |
|--------|-----------|--------|
| RSI(14) | 50-70 = 7pts (healthy uptrend), 40-50 = 4pts (neutral), 30-40 = 2pts (oversold bounce possible), <30 = 1pt (deeply oversold), >70 = 3pts (overbought risk) | 0–7 |
| MACD above signal | Bullish crossover | +6 |
| MACD histogram rising | Momentum accelerating | +6 |
| Price Rate of Change (20d) | > 5% = 6pts, > 0% = 3pts, < 0% = 0pts | 0–6 |
| **Pillar Total** | | **0–25** |

**RSI Calculation:**
```
# RSI(14) — standard Wilder's smoothing
changes = [closes[i] - closes[i-1] for i in range(1, len(closes))]
gains = [max(c, 0) for c in changes]
losses = [abs(min(c, 0)) for c in changes]

avg_gain = wilder_smooth(gains, 14)
avg_loss = wilder_smooth(losses, 14)

rs = avg_gain / avg_loss
rsi = 100 - (100 / (1 + rs))
```

**MACD Calculation:**
```
# MACD(12, 26, 9)
ema_12 = exponential_moving_average(closes, 12)
ema_26 = exponential_moving_average(closes, 26)
macd_line = ema_12 - ema_26
signal_line = exponential_moving_average(macd_line, 9)
histogram = macd_line - signal_line

momentum_score = 0

# RSI scoring
if 50 <= rsi <= 70: momentum_score += 7
elif 40 <= rsi < 50: momentum_score += 4
elif 30 <= rsi < 40: momentum_score += 2
elif rsi < 30: momentum_score += 1
elif rsi > 70: momentum_score += 3

# MACD
if macd_line > signal_line: momentum_score += 6
if histogram > histogram_prev: momentum_score += 6

# Rate of Change
roc_20 = (price - closes[-20]) / closes[-20]
if roc_20 > 0.05: momentum_score += 6
elif roc_20 > 0: momentum_score += 3

momentum_score = min(momentum_score, 25)
```

### Pillar C: Price Action (0–25 points)

| Signal | Condition | Points |
|--------|-----------|--------|
| 52-Week Position | Top 20% = 7pts, Top 40% = 5pts, Mid = 3pts, Bottom 40% = 1pt, Bottom 20% = 0 | 0–7 |
| Distance from 52W High | Within 5% = 6pts, within 10% = 4pts, within 20% = 2pts | 0–6 |
| 1-Month Trend | Up > 5% = 6pts, Up > 0% = 3pts, Down = 0 | 0–6 |
| Higher Lows (20d) | Recent low > prior low (uptrend structure) | +6 |
| **Pillar Total** | | **0–25** |

**Scoring logic:**
```
price_action_score = 0

# 52-week position (where price sits in the range)
week52_range = fifty_two_week_high - fifty_two_week_low
position = (price - fifty_two_week_low) / week52_range  # 0.0 to 1.0

if position > 0.80: price_action_score += 7
elif position > 0.60: price_action_score += 5
elif position > 0.40: price_action_score += 3
elif position > 0.20: price_action_score += 1

# Distance from 52-week high
distance_from_high = (fifty_two_week_high - price) / fifty_two_week_high
if distance_from_high < 0.05: price_action_score += 6
elif distance_from_high < 0.10: price_action_score += 4
elif distance_from_high < 0.20: price_action_score += 2

# 1-month trend
month_ago_price = closes[-22]  # ~22 trading days
monthly_change = (price - month_ago_price) / month_ago_price
if monthly_change > 0.05: price_action_score += 6
elif monthly_change > 0: price_action_score += 3

# Higher lows check (uptrend structure)
recent_low = min(lows[-10:])
prior_low = min(lows[-20:-10])
if recent_low > prior_low: price_action_score += 6

price_action_score = min(price_action_score, 25)
```

### Pillar D: Volume Trend (0–25 points)

| Signal | Condition | Points |
|--------|-----------|--------|
| Volume vs 3M Average | Above avg on up days = 8pts | 0–8 |
| Volume Trend (20d) | Rising volume = 6pts, stable = 3pts | 0–6 |
| Accumulation Pattern | Up days volume > down days volume (20d) = 6pts | 0–6 |
| On-Balance Volume trend | OBV rising over 20d | +5 |
| **Pillar Total** | | **0–25** |

**Scoring logic:**
```
volume_score = 0

# Current volume vs 3-month average
if current_volume > avg_volume_3m * 1.2:
    # High volume — check if it's on an up day
    if price_change_today > 0: volume_score += 8
    else: volume_score += 2  # high volume selling is bearish signal
elif current_volume > avg_volume_3m * 0.8:
    volume_score += 4  # normal volume

# Volume trend over 20 days
avg_vol_recent = average(volumes[-10:])
avg_vol_prior = average(volumes[-20:-10])
if avg_vol_recent > avg_vol_prior * 1.1: volume_score += 6
elif avg_vol_recent > avg_vol_prior * 0.9: volume_score += 3

# Accumulation: compare volume on up days vs down days
up_day_volume = sum(v for v, c in zip(volumes[-20:], changes[-20:]) if c > 0)
down_day_volume = sum(v for v, c in zip(volumes[-20:], changes[-20:]) if c < 0)
if up_day_volume > down_day_volume * 1.2: volume_score += 6
elif up_day_volume > down_day_volume: volume_score += 3

# On-Balance Volume (OBV) trend
obv = calculate_obv(closes, volumes)
obv_sma = average(obv[-20:])
obv_sma_prev = average(obv[-40:-20])
if obv_sma > obv_sma_prev: volume_score += 5

volume_score = min(volume_score, 25)
```

### Total Technical Score

```
TECHNICAL_SCORE = ma_score + momentum_score + price_action_score + volume_score
# Range: 0–100
```

---

## Step 5 — Calculate Composite Score and Signal

### Composite Weighting

Fundamentals are weighted more heavily because they drive long-term value:

```
COMPOSITE_SCORE = (FUNDAMENTAL_SCORE × 0.60) + (TECHNICAL_SCORE × 0.40)
# Range: 0–100
```

### Signal Mapping — 7-Tier Rating

| Composite Score | Signal | Label | Description |
|----------------|--------|-------|-------------|
| **85–100** | ✅✅✅ | **MUST BUY** | Exceptional fundamentals + strong uptrend. Rare — act decisively. |
| **70–84** | ✅✅ | **STRONG BUY** | Very good fundamentals with confirming trend. High conviction entry. |
| **55–69** | ✅ | **BUY** | Solid fundamentals, acceptable trend. Good entry on dips. |
| **40–54** | ⚪ | **HOLD** | Mixed signals. Hold existing positions, wait for clarity. |
| **25–39** | ❌ | **SELL** | Weakening fundamentals or broken trend. Consider reducing. |
| **10–24** | ❌❌ | **STRONG SELL** | Poor fundamentals with downtrend. Exit most positions. |
| **0–9** | ❌❌❌ | **MUST SELL** | Severely distressed. Financial health at risk. Exit immediately. |

### Trend Arrow Enhancement

The trend direction from the technical score adds context to the signal:

| Technical Score | Trend Arrow | Meaning |
|----------------|-------------|---------|
| **75–100** | ▲▲ | Strong uptrend |
| **50–74** | ▲ | Uptrend |
| **40–49** | ► | Sideways / neutral |
| **25–39** | ▼ | Downtrend |
| **0–24** | ▼▼ | Strong downtrend |

---

## Step 6 — Output Format

### Single Stock Analysis

```
╔══════════════════════════════════════════════════════════════╗
║  📊  FUNDAMENTAL TREND ANALYSIS — NVDA                      ║
║  NVIDIA Corporation                                          ║
║  Price: $875.40  ▲ +2.34%  |  {date}                        ║
╠══════════════════════════════════════════════════════════════╣
║                                                              ║
║  ══ SIGNAL ══                                                ║
║                                                              ║
║     ✅✅  STRONG BUY                                         ║
║     Composite Score: 76/100  |  Trend: ▲▲ Strong Uptrend    ║
║                                                              ║
╠══════════════════════════════════════════════════════════════╣
║                                                              ║
║  ══ FUNDAMENTAL SCORE: 72/100 ══                             ║
║                                                              ║
║  Profitability     ████████████████████░░░░░  20/25          ║
║    ROE: 91.2%  (7)  ROA: 54.1%  (6)                         ║
║    Margin: 55.0% (6)  OCF Quality (6)                        ║
║                                                              ║
║  Valuation         ██████████░░░░░░░░░░░░░░░  10/25          ║
║    P/E: 65.2  (1)  Fwd P/E: 38.1  (4)                       ║
║    P/B: 45.8  (0)  PEG: 1.2  (3)  EV/EBITDA: 52  (0)       ║
║                                                              ║
║  Financial Health  ██████████████████████░░░  22/25          ║
║    D/E: 17.2%  (7)  Current: 4.2  (6)                       ║
║    Cash/Debt: 2.1  (6)  Coverage: 85x  (6)                   ║
║                                                              ║
║  Growth            ████████████████████░░░░░  20/25          ║
║    Rev Growth: 122%  (7)  EPS Growth: 581%  (6)             ║
║    Qtr Growth: 109%  (6)  Analyst 5Y: 35%  (6)              ║
║                                                              ║
╠══════════════════════════════════════════════════════════════╣
║                                                              ║
║  ══ TECHNICAL SCORE: 82/100 ══                               ║
║                                                              ║
║  MA Alignment      ██████████████████████████  25/25          ║
║    Price > SMA20 ✓   Price > SMA50 ✓   Price > SMA200 ✓     ║
║    SMA50 > SMA200 ✓ (Golden Cross)  SMA50 Rising ✓          ║
║                                                              ║
║  Momentum          █████████████████████░░░░  21/25          ║
║    RSI(14): 62  (7)  MACD: Bullish (6)                       ║
║    Histogram: Rising (6)  ROC(20d): +3.2% (3)               ║
║                                                              ║
║  Price Action      ████████████████████░░░░░  20/25          ║
║    52W Position: 85%  (7)  From High: -4%  (6)              ║
║    1M Trend: +8.2%  (6)  Higher Lows: No (0)                ║
║                                                              ║
║  Volume            ████████████████░░░░░░░░░  16/25          ║
║    Vol vs Avg: 1.3x ↑  (8)  Trend: Stable  (3)             ║
║    Accumulation: Yes (6)  OBV: Flat (0)                      ║
║                                                              ║
╠══════════════════════════════════════════════════════════════╣
║  ══ COMPOSITE BREAKDOWN ══                                   ║
║  Fundamental: 72 × 0.60 = 43.2                              ║
║  Technical:   82 × 0.40 = 32.8                              ║
║  COMPOSITE:   76.0 / 100                                     ║
║                                                              ║
║  ⚠ Note: High growth but elevated valuation. Strong trend    ║
║    confirms momentum. Watch for P/E compression risk.        ║
╚══════════════════════════════════════════════════════════════╝
```

### Multi-Stock Comparison Table

When analyzing multiple tickers (e.g., a portfolio):

```
╔═══════════════════════════════════════════════════════════════════════════════╗
║  📊  PORTFOLIO ANALYSIS — Fundamental + Trend Scores                         ║
║  {date}                                                                      ║
╠═══════╦════════╦══════╦══════╦══════╦══════╦══════╦══════╦═══════════════════╣
║ TICKER║ PRICE  ║ FUND ║ TECH ║ COMP ║ TREND║SIGNAL║      ║ KEY FACTOR       ║
╠═══════╬════════╬══════╬══════╬══════╬══════╬══════╬══════╬═══════════════════╣
║ NVDA  ║ 875.40 ║  72  ║  82  ║  76  ║  ▲▲  ║ ✅✅ ║ STRG ║ Growth monster   ║
║ MSFT  ║ 420.15 ║  78  ║  65  ║  73  ║  ▲   ║ ✅✅ ║ STRG ║ Cash machine     ║
║ AAPL  ║ 182.30 ║  75  ║  52  ║  66  ║  ▲   ║ ✅   ║ BUY  ║ Stable margins   ║
║ GOOGL ║ 155.80 ║  70  ║  58  ║  65  ║  ▲   ║ ✅   ║ BUY  ║ Value + growth   ║
║ TSLA  ║ 175.20 ║  45  ║  71  ║  55  ║  ▲▲  ║ ✅   ║ BUY  ║ Trend > fundmntl ║
║ META  ║ 505.30 ║  80  ║  74  ║  78  ║  ▲▲  ║ ✅✅ ║ STRG ║ Top value+growth ║
║ V     ║ 280.10 ║  82  ║  60  ║  73  ║  ▲   ║ ✅✅ ║ STRG ║ Quality leader   ║
║ AMD   ║ 165.40 ║  48  ║  42  ║  46  ║  ►   ║ ⚪   ║ HOLD ║ Valuation stretch║
║ AMZN  ║ 186.90 ║  65  ║  68  ║  66  ║  ▲   ║ ✅   ║ BUY  ║ Margin expansion ║
║ AVGO  ║ 1520.0 ║  74  ║  78  ║  76  ║  ▲▲  ║ ✅✅ ║ STRG ║ AI + dividends   ║
╠═══════╩════════╩══════╩══════╩══════╩══════╩══════╩══════╩═══════════════════╣
║ Legend: FUND=Fundamental(0-100) TECH=Technical(0-100) COMP=Composite(0-100) ║
║ Signals: ✅✅✅ Must Buy  ✅✅ Strong Buy  ✅ Buy  ⚪ Hold                    ║
║          ❌ Sell  ❌❌ Strong Sell  ❌❌❌ Must Sell                           ║
║ Trend:   ▲▲ Strong Up  ▲ Up  ► Sideways  ▼ Down  ▼▼ Strong Down           ║
╚═══════════════════════════════════════════════════════════════════════════════╝
```

---

## Step 7 — Contextual Notes Generation

After computing scores, generate 1–2 lines of context based on the score pattern:

| Pattern | Note |
|---------|------|
| High Fundamental + Low Technical | "Strong company in a downtrend — potential value opportunity. Wait for trend reversal." |
| Low Fundamental + High Technical | "Momentum play with weak fundamentals — high risk. Use tight stop-loss." |
| High Fundamental + High Technical | "Quality + trend alignment — high conviction entry." |
| Low Fundamental + Low Technical | "Avoid — deteriorating business with no trend support." |
| High Valuation + High Growth | "Premium valuation justified by growth — watch for deceleration." |
| Low D/E + High ROE | "Capital-efficient compounder — Buffett-style quality." |
| Negative Earnings Growth + High P/E | "Market pricing in turnaround — high risk if recovery doesn't materialize." |

---

## Sector-Aware Adjustments

Different sectors have different "normal" ranges for metrics. Apply these adjustments before scoring:

| Sector | P/E Adjustment | D/E Adjustment | Growth Adj | Notes |
|--------|---------------|----------------|------------|-------|
| Technology | P/E thresholds × 1.5 | Standard | Growth thresholds × 1.3 | Higher multiples normal |
| Financials | Standard | D/E thresholds × 3.0 | Standard | Banks carry more leverage |
| Utilities | P/E thresholds × 0.8 | D/E thresholds × 1.5 | Growth thresholds × 0.5 | Slow growth, regulated |
| Healthcare | P/E thresholds × 1.3 | Standard | Standard | Pipeline optionality |
| REITs | Skip P/E, use P/FFO | D/E thresholds × 2.0 | Growth thresholds × 0.7 | Leverage is structural |
| Consumer Staples | Standard | Standard | Growth thresholds × 0.7 | Defensive, lower growth |
| Energy | P/E thresholds × 0.8 | Standard | Standard | Cyclical earnings |

To detect sector, use the `assetProfile` module:

```bash
curl -s "https://query2.finance.yahoo.com/v10/finance/quoteSummary/${TICKER}?modules=assetProfile" \
  -H "User-Agent: Mozilla/5.0" | jq '.quoteSummary.result[0].assetProfile.sector'
```

---

## Batch Analysis — Multiple Tickers

For portfolio analysis, fetch quotes in batch and fundamentals per-ticker:

```bash
# Batch quotes (up to 100 tickers)
TICKERS="NVDA,MSFT,AAPL,GOOGL,TSLA,META,V,AMD,AMZN,AVGO"
curl -s "https://query2.finance.yahoo.com/v7/finance/quote?symbols=${TICKERS}" \
  -H "User-Agent: Mozilla/5.0" | jq '.quoteResponse.result[]'

# Then for each ticker, fetch fundamentals (can be done concurrently)
for TICKER in NVDA MSFT AAPL GOOGL TSLA META V AMD AMZN AVGO; do
  curl -s "https://query2.finance.yahoo.com/v10/finance/quoteSummary/${TICKER}?modules=financialData,defaultKeyStatistics,summaryDetail,earningsTrend,assetProfile" \
    -H "User-Agent: Mozilla/5.0" > "/tmp/${TICKER}_fundamentals.json" &
done
wait
```

---

## Quick Reference — Scoring Thresholds

### Fundamental Metrics Cheat Sheet

| Metric | Excellent | Good | Fair | Poor |
|--------|-----------|------|------|------|
| ROE | > 15% | 10-15% | 5-10% | < 5% |
| ROA | > 10% | 5-10% | 2-5% | < 2% |
| Profit Margin | > 20% | 10-20% | 5-10% | < 5% |
| P/E Ratio | < 15 | 15-20 | 20-30 | > 30 |
| PEG Ratio | < 1.0 | 1.0-1.5 | 1.5-2.0 | > 2.0 |
| Debt/Equity | < 30% | 30-50% | 50-100% | > 100% |
| Current Ratio | > 2.0 | 1.5-2.0 | 1.0-1.5 | < 1.0 |
| Revenue Growth | > 25% | 15-25% | 5-15% | < 5% |

### Technical Signals Cheat Sheet

| Indicator | Bullish | Neutral | Bearish |
|-----------|---------|---------|---------|
| Price vs SMA200 | Above | At | Below |
| SMA50 vs SMA200 | Golden Cross | Converging | Death Cross |
| RSI(14) | 50-70 | 40-50 | < 30 or > 70 |
| MACD | Above signal | At signal | Below signal |
| Volume | Rising on up days | Average | Rising on down days |

---

## Interpreting Results

**High composite scores (70+)** indicate alignment between business quality and market momentum — these are the highest conviction opportunities.

**Divergence between fundamental and technical** scores is informative:
- Fundamental ≫ Technical: **Value trap risk** or **turnaround candidate**. The business is strong but the market disagrees. Look for catalysts.
- Technical ≫ Fundamental: **Momentum play**. The market is running ahead of fundamentals. Risk of mean reversion. Use trailing stops.

**Best opportunities** often emerge when:
1. Fundamental score > 65 AND technical score was < 40 but is now rising (trend reversal on a quality name)
2. Both scores > 70 (quality + momentum alignment)
3. Fundamental score improving quarter-over-quarter (business inflection)

**Worst situations** (must sell territory):
1. Both scores < 25 (deteriorating business in a downtrend)
2. Financial health score < 5 (potential bankruptcy risk — Altman Z-Score territory)
3. Fundamental declining AND technical breaking key support levels

---

## Limitations and Disclaimers

- **Not financial advice.** This is a quantitative screening tool. Always do your own research.
- **Backward-looking.** Fundamentals are based on reported financials (1–3 months lag). Markets are forward-looking.
- **Sector differences.** A P/E of 30 is expensive for a utility but cheap for a high-growth tech company. The sector adjustments help but aren't perfect.
- **Data availability.** Some metrics may be missing for newer companies, ADRs, or small caps. Score only on available data and note missing pillars.
- **No macro overlay.** This model doesn't account for interest rates, geopolitical risk, or market-wide conditions. Combine with market-overview skill for context.

---

## References

- [Piotroski F-Score (Wikipedia)](https://en.wikipedia.org/wiki/Piotroski_F-score) — Original 9-criteria financial strength scoring
- [Altman Z-Score](https://eodhd.com/financial-academy/financial-faq/how-to-calculate-altman-z-score-or-piotrosky-score) — Bankruptcy prediction model
- [Schwab: Five Key Financial Ratios](https://www.schwab.com/learn/story/five-key-financial-ratios-stock-analysis) — Core ratio analysis
- [Danelfin AI Score](https://danelfin.com/stock/IMO?score=fundamental) — AI-powered fundamental scoring system
- [Tickeron AI Ratings](https://tickeron.com/) — Combined fundamental + technical scoring
- [Financial Modeling Prep API](https://site.financialmodelingprep.com/developer/docs) — Free fundamental data APIs
- [Yahoo Finance](https://finance.yahoo.com/) — Data source for all calculations
