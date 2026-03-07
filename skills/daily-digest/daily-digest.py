#!/usr/bin/env python3
"""
Daily Digest — Unified Newsletter
Sections:
  1. Fundamental Trend Analysis (FTA) — key stocks scored
  2. Market Overview — indices, commodities, crypto, currencies, movers
  3. Tech & Market Intel — top news from RSS feeds

Usage: python3 daily-digest.py
"""

import yfinance as yf
import feedparser
import smtplib
import sys
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from datetime import datetime, timezone, timedelta
import time

# ── Config ─────────────────────────────────────────────────────────────────────
SMTP_HOST = "smtp.gmail.com"
SMTP_PORT = 465
SMTP_USER = os.environ.get("SMTP_USER", "dengalebr@gmail.com")
SMTP_PASS = os.environ.get("SMTP_PASS", "")
RECIPIENTS = os.environ.get("SMTP_RECIPIENTS", "dengalebr@gmail.com,badengal@visa.com").split(",")

SGT = timezone(timedelta(hours=8))
NOW = datetime.now(SGT)
DATE_STR = NOW.strftime("%a %d %b %Y")
TIME_STR = NOW.strftime("%I:%M %p SGT")

# FTA tickers — live prices via yfinance (MFs/SGB handled separately as static NAV rows)
FTA_TICKERS = [
    # 🇺🇸 US Stocks
    "V", "NVDA", "NFLX", "MSFT", "AAPL", "GOOGL", "META", "AVGO", "AMD", "PLTR",
    # 🇺🇸 US ETFs
    "VOO", "QQQ", "SPY", "SPMO", "IBIT",
    # 🇸🇬 Singapore
    "Z74.SI", "C6L.SI", "HST.SI", "D05.SI",
    # 🇮🇳 India Stocks & Index
    "ITC.NS", "RELIANCE.NS", "TMCV.NS", "^NSEI",
    # 🇮🇳 Gold proxy (for SGB context)
    "GC=F",
]

FTA_NAMES = {
    # US Stocks
    "V": "Visa", "NVDA": "NVIDIA", "NFLX": "Netflix", "MSFT": "Microsoft",
    "AAPL": "Apple", "GOOGL": "Alphabet", "META": "Meta", "AVGO": "Broadcom",
    "AMD": "AMD", "PLTR": "Palantir",
    # US ETFs
    "VOO": "Vanguard S&P 500", "QQQ": "Invesco QQQ", "SPY": "SPDR S&P 500",
    "SPMO": "S&P 500 Momentum", "IBIT": "iShares Bitcoin",
    # Singapore
    "Z74.SI": "Singtel", "C6L.SI": "SIA", "HST.SI": "Lion HSTECH", "D05.SI": "DBS",
    # India
    "ITC.NS": "ITC", "RELIANCE.NS": "Reliance", "TMCV.NS": "Tata Motors",
    "^NSEI": "NIFTY 50", "GC=F": "Gold / SGB",
}



# Portfolio holdings (from PORTFOLIO.json)
PORTFOLIO = [
    {"name": "Parag Parikh Flexi Cap", "ticker": None, "region": "India", "currency": "INR", "value_inr": 5431757, "type": "mutual_fund"},
    {"name": "Mirae Asset Large & Midcap", "ticker": None, "region": "India", "currency": "INR", "value_inr": 4073361, "type": "mutual_fund"},
    {"name": "Axis Midcap Dir-G", "ticker": None, "region": "India", "currency": "INR", "value_inr": 1130664, "type": "mutual_fund"},
    {"name": "SGB Bonds", "ticker": None, "region": "India", "currency": "INR", "value_inr": 1078533, "type": "fixed_income"},
    {"name": "SGB Aug 27", "ticker": "GC=F", "region": "India", "currency": "INR", "quantity_grams": 70, "type": "gold_bond"},
    {"name": "Vanguard S&P 500 (VOO)", "ticker": "VOO", "region": "USA", "currency": "USD", "quantity": 22.2676, "avg_price": 631.15, "type": "etf"},
    {"name": "Invesco QQQ", "ticker": "QQQ", "region": "India", "currency": "USD", "quantity": 19.3734, "avg_price": 608.81, "type": "etf"},
    {"name": "Singapore Cash/MM", "ticker": None, "region": "Singapore", "currency": "SGD", "value_sgd": 13204, "type": "cash"},
]
USD_INR = 86.0   # approximate, will fetch live
USD_SGD = 1.274

# Market overview tickers
INDICES = {
    "^GSPC": "S&P 500 🇺🇸", "^IXIC": "NASDAQ 🇺🇸", "^STI": "STI Singapore 🇸🇬",
    "^BSESN": "Sensex 🇮🇳", "^NSEI": "Nifty 50 🇮🇳", "^HSI": "Hang Seng 🇭🇰"
}
COMMODITIES = {
    "GC=F": "🥇 Gold", "SI=F": "🥈 Silver", "CL=F": "🛢️ WTI Oil", "BZ=F": "🛢️ Brent"
}
CRYPTO = {"BTC-USD": "₿ Bitcoin", "ETH-USD": "Ξ Ethereum"}
FOREX = {"INR=X": "USD/INR", "SGD=X": "USD/SGD", "SGDINR=X": "SGD/INR"}
US_MOVERS = ["NVDA", "AAPL", "MSFT", "GOOGL", "META", "TSLA", "AMZN", "V", "NFLX", "AMD", "AVGO", "ORCL"]
SG_MOVERS = ["D05.SI", "O39.SI", "U11.SI", "C6L.SI", "Z74.SI", "S58.SI"]

US_NAMES = {
    "NVDA": "NVIDIA", "AAPL": "Apple", "MSFT": "Microsoft", "GOOGL": "Alphabet",
    "META": "Meta", "TSLA": "Tesla", "AMZN": "Amazon", "V": "Visa",
    "NFLX": "Netflix", "AMD": "AMD", "AVGO": "Broadcom", "ORCL": "Oracle"
}
SG_NAMES = {
    "D05.SI": "DBS Bank", "O39.SI": "OCBC", "U11.SI": "UOB",
    "C6L.SI": "SIA", "Z74.SI": "SingTel", "S58.SI": "SATS"
}

# RSS feeds
RSS_FEEDS = [
    ("TechCrunch", "https://techcrunch.com/feed/"),
    ("Ars Technica", "https://feeds.arstechnica.com/arstechnica/index"),
    ("The Verge", "https://www.theverge.com/rss/index.xml"),
    ("Wired", "https://www.wired.com/feed/rss"),
    ("CNBC Top News", "https://www.cnbc.com/id/100003114/device/rss/rss.html"),
    ("CNBC World", "https://www.cnbc.com/id/100727362/device/rss/rss.html"),
    ("MarketWatch", "https://feeds.content.dowjones.io/public/rss/mw_topstories"),
    ("Yahoo Finance", "https://finance.yahoo.com/news/rssindex"),
    ("BBC Business", "https://feeds.bbci.co.uk/news/business/rss.xml"),
    ("NPR Business", "https://feeds.npr.org/1006/rss.xml"),
    ("Hacker News", "https://hnrss.org/frontpage"),
]

KEYWORDS_HIGH = ["acquisition","merger","semiconductor","disruption","revolutionary","ipo","bankruptcy"]
KEYWORDS_MED = ["ai","artificial intelligence","chatgpt","openai","anthropic","nvidia","fintech",
                "regulation","cybersecurity","quantum","data breach","breach","hack","fed","inflation",
                "interest rate","earnings","revenue","profit"]
KEYWORDS_STD = ["startup","funding","blockchain","crypto","bitcoin","apple","google","microsoft",
                "amazon","tesla","meta","netflix","visa"]

# ── FTA Scoring ────────────────────────────────────────────────────────────────
def calc_fundamental(info):
    roe = info.get('returnOnEquity') or 0
    roa = info.get('returnOnAssets') or 0
    margin = info.get('profitMargins') or 0
    ocf = info.get('operatingCashflow') or 0
    ni = info.get('netIncomeToCommon') or 0
    p = 0
    if roe > 0.15: p += 7
    elif roe > 0.10: p += 5
    elif roe > 0.05: p += 3
    elif roe > 0: p += 1
    if roa > 0.10: p += 6
    elif roa > 0.05: p += 4
    elif roa > 0.02: p += 2
    elif roa > 0: p += 1
    if margin > 0.20: p += 6
    elif margin > 0.10: p += 4
    elif margin > 0.05: p += 2
    elif margin > 0: p += 1
    if ocf > 0: p += 3
    if ocf > ni: p += 3
    p = min(p, 25)

    pe = info.get('trailingPE') or 0
    fpe = info.get('forwardPE') or 0
    pb = info.get('priceToBook') or 0
    peg = info.get('pegRatio') or 0
    evebitda = info.get('enterpriseToEbitda') or 0
    v = 0
    if 0 < pe < 15: v += 7
    elif 0 < pe < 20: v += 5
    elif 0 < pe < 30: v += 3
    elif 0 < pe < 50: v += 1
    if fpe > 0 and pe > 0:
        if fpe < pe * 0.90: v += 4
        elif fpe < pe: v += 2
    if 0 < pb < 1.5: v += 5
    elif pb < 3: v += 3
    elif pb < 5: v += 2
    elif pb < 10: v += 1
    if 0 < peg < 1.0: v += 5
    elif peg < 1.5: v += 3
    elif peg < 2.0: v += 2
    elif peg < 3.0: v += 1
    if 0 < evebitda < 10: v += 4
    elif evebitda < 15: v += 3
    elif evebitda < 20: v += 2
    elif evebitda < 30: v += 1
    v = min(v, 25)

    de = info.get('debtToEquity') or 0
    cr = info.get('currentRatio') or 0
    cash = info.get('totalCash') or 0
    debt = info.get('totalDebt') or 1
    h = 0
    if de < 30: h += 7
    elif de < 50: h += 5
    elif de < 100: h += 3
    elif de < 200: h += 1
    if cr > 2.0: h += 6
    elif cr > 1.5: h += 4
    elif cr > 1.0: h += 2
    cash_ratio = cash / debt if debt > 0 else 2
    if cash_ratio > 1.0: h += 6
    elif cash_ratio > 0.5: h += 4
    elif cash_ratio > 0.25: h += 2
    elif cash_ratio > 0: h += 1
    h += 4
    h = min(h, 25)

    rg = info.get('revenueGrowth') or 0
    eg = info.get('earningsGrowth') or 0
    qeg = info.get('earningsQuarterlyGrowth') or 0
    g = 0
    if rg > 0.25: g += 7
    elif rg > 0.15: g += 5
    elif rg > 0.05: g += 3
    elif rg > 0: g += 1
    if eg > 0.25: g += 6
    elif eg > 0.15: g += 4
    elif eg > 0.05: g += 2
    elif eg > 0: g += 1
    if qeg > 0.20: g += 6
    elif qeg > 0.10: g += 4
    elif qeg > 0: g += 2
    g += 3
    g = min(g, 25)
    return p, v, h, g, min(p+v+h+g, 100)

def calc_technical(closes, price):
    closes = [c for c in closes if c]
    if len(closes) < 200:
        return 0, 0, 0, 0, 0
    sma20 = sum(closes[-20:]) / 20
    sma50 = sum(closes[-50:]) / 50
    sma200 = sum(closes[-200:]) / 200
    sma50_prev = sum(closes[-70:-20]) / 50
    ma = 0
    if price > sma50: ma += 5
    if price > sma200: ma += 5
    if sma50 > sma200: ma += 7
    if price > sma20: ma += 4
    if sma50 > sma50_prev: ma += 4
    ma = min(ma, 25)
    changes = [closes[i]-closes[i-1] for i in range(1, len(closes))]
    gains = [max(c,0) for c in changes]
    losses = [abs(min(c,0)) for c in changes]
    ag = sum(gains[-14:]) / 14
    al = sum(losses[-14:]) / 14
    rs = ag / al if al > 0 else 100
    rsi = 100 - (100 / (1 + rs))
    mo = 0
    if 50 <= rsi <= 70: mo += 7
    elif 40 <= rsi < 50: mo += 4
    elif 30 <= rsi < 40: mo += 2
    elif rsi < 30: mo += 1
    elif rsi > 70: mo += 3
    roc20 = (price - closes[-22]) / closes[-22] if len(closes) >= 22 else 0
    if roc20 > 0.05: mo += 6
    elif roc20 > 0: mo += 3
    mo += 6
    mo = min(mo, 25)
    hi52 = max(closes)
    lo52 = min(closes)
    pos = (price - lo52) / (hi52 - lo52) if (hi52 - lo52) > 0 else 0.5
    pa = 0
    if pos > 0.80: pa += 7
    elif pos > 0.60: pa += 5
    elif pos > 0.40: pa += 3
    elif pos > 0.20: pa += 1
    dist = (hi52 - price) / hi52
    if dist < 0.05: pa += 6
    elif dist < 0.10: pa += 4
    elif dist < 0.20: pa += 2
    m1 = (price - closes[-22]) / closes[-22] if len(closes) >= 22 else 0
    if m1 > 0.05: pa += 6
    elif m1 > 0: pa += 3
    pa = min(pa, 25)
    return ma, mo, pa, 10, min(ma + mo + pa + 10, 100)

def signal(comp):
    if comp >= 85: return "✅✅✅", "MUST BUY", "#166534", "#dcfce7"
    elif comp >= 70: return "✅✅", "STRONG BUY", "#15803d", "#f0fdf4"
    elif comp >= 55: return "✅", "BUY", "#16a34a", "#f0fdf4"
    elif comp >= 40: return "⚪", "HOLD", "#92400e", "#fffbeb"
    elif comp >= 25: return "❌", "SELL", "#dc2626", "#fff1f2"
    elif comp >= 10: return "❌❌", "STRONG SELL", "#991b1b", "#fef2f2"
    else: return "❌❌❌", "MUST SELL", "#7f1d1d", "#fef2f2"

def trend_arrow(tech):
    if tech >= 75: return "▲▲"
    elif tech >= 50: return "▲"
    elif tech >= 40: return "►"
    elif tech >= 25: return "▼"
    else: return "▼▼"

# ── Market Data ────────────────────────────────────────────────────────────────
def fetch_quote(ticker):
    try:
        tk = yf.Ticker(ticker)
        info = tk.info
        hist = tk.history(period="5d")
        if hist.empty:
            return None
        price = info.get('regularMarketPrice') or float(hist['Close'].iloc[-1])
        prev = float(hist['Close'].iloc[-2]) if len(hist) >= 2 else price
        chg_pct = (price - prev) / prev * 100
        # Week change
        hist_1m = tk.history(period="1mo")
        week_chg = None
        if len(hist_1m) >= 6:
            week_chg = (price - float(hist_1m['Close'].iloc[-6])) / float(hist_1m['Close'].iloc[-6]) * 100
        return {'price': price, 'chg': chg_pct, 'week_chg': week_chg, 'name': info.get('shortName','')[:20]}
    except Exception as e:
        return None

def fmt_price(p, ticker=""):
    if any(x in ticker for x in ["=X", "^BSESN", "^NSEI", "^STI", "^GSPC", "^IXIC", "^HSI"]):
        return f"{p:,.0f}"
    return f"${p:,.2f}"

def fmt_chg(c):
    if c is None: return "—"
    color = "#22c55e" if c >= 0 else "#ef4444"
    sign = "+" if c >= 0 else ""
    return f'<span style="color:{color};font-weight:600">{sign}{c:.1f}%</span>'

# ── RSS Intel ──────────────────────────────────────────────────────────────────
def score_article(title, desc):
    text = (title + " " + (desc or "")).lower()
    score = 0
    for kw in KEYWORDS_HIGH:
        if kw in text: score += (10 if kw in title.lower() else 5)
    for kw in KEYWORDS_MED:
        if kw in text: score += (7 if kw in title.lower() else 4)
    for kw in KEYWORDS_STD:
        if kw in text: score += (4 if kw in title.lower() else 2)
    return score

def fetch_news():
    articles = []
    cutoff = datetime.now(timezone.utc) - timedelta(hours=28)
    for name, url in RSS_FEEDS:
        try:
            feed = feedparser.parse(url)
            for entry in feed.entries[:30]:
                published = None
                if hasattr(entry, 'published_parsed') and entry.published_parsed:
                    published = datetime(*entry.published_parsed[:6], tzinfo=timezone.utc)
                if published and published < cutoff:
                    continue
                title = entry.get('title', '')
                desc = entry.get('summary', '')
                link = entry.get('link', '')
                score = score_article(title, desc)
                if score > 0:
                    articles.append({'title': title, 'link': link, 'source': name, 'score': score, 'published': published})
        except:
            pass
    # Dedup by title similarity
    seen = set()
    unique = []
    for a in sorted(articles, key=lambda x: -x['score']):
        key = a['title'][:50].lower()
        if key not in seen:
            seen.add(key)
            unique.append(a)
    return unique[:20]

# ── HTML Builders ──────────────────────────────────────────────────────────────
def build_fta_section(rows):
    # Group by region
    sg_tickers = {"Z74.SI", "C6L.SI", "HST.SI", "D05.SI"}
    etf_tickers = {"VOO", "QQQ", "SPY", "SPMO", "IBIT"}
    india_tickers = {"ITC.NS", "RELIANCE.NS", "TMCV.NS", "^NSEI", "GC=F"}
    groups = [
        ("🇺🇸 US Stocks", [r for r in rows if r["ticker"] not in sg_tickers and r["ticker"] not in etf_tickers and r["ticker"] not in india_tickers]),
        ("📊 US ETFs", [r for r in rows if r["ticker"] in etf_tickers]),
        ("🇸🇬 Singapore", [r for r in rows if r["ticker"] in sg_tickers]),
        ("🇮🇳 India", [r for r in rows if r["ticker"] in india_tickers]),
    ]

    rows_html = ""
    for group_label, group_rows in groups:
        if not group_rows:
            continue
        rows_html += f'<tr style="background:#eef2ff"><td colspan="7" style="padding:7px 12px;font-weight:700;color:#0c2461;font-size:11px;letter-spacing:.5px;text-transform:uppercase">{group_label}</td></tr>'
        for r in group_rows:
            chg_color = "#22c55e" if r['chg'] >= 0 else "#ef4444"
            chg_str = f"+{r['chg']:.1f}%" if r['chg'] >= 0 else f"{r['chg']:.1f}%"
            rows_html += f"""<tr style="background:{r['bg']};border-bottom:1px solid #f0f0f0">
              <td style="padding:8px 12px;font-weight:700;color:#1e293b">{r['ticker']}<br>
                <span style="font-size:10px;font-weight:400;color:#94a3b8">{r['name']}</span></td>
              <td style="padding:8px 8px;text-align:right;font-weight:600">{fmt_price(r['price'])}<br>
                <span style="font-size:10px;color:{chg_color}">{chg_str}</span></td>
              <td style="padding:8px 8px;text-align:center">
                <span style="font-size:15px;font-weight:800;color:#1e293b">{r['fund']}</span><br>
                <span style="font-size:9px;color:#94a3b8">P{r['p']} V{r['v']} H{r['h']} G{r['g']}</span></td>
              <td style="padding:8px 8px;text-align:center;font-size:15px;font-weight:800;color:#1e293b">{r['tech']}</td>
              <td style="padding:8px 8px;text-align:center">
                <div style="background:#e5e7eb;border-radius:6px;height:7px;width:64px;margin:0 auto 3px auto">
                  <div style="background:{r['color']};height:7px;border-radius:6px;width:{r['comp']}%"></div></div>
                <span style="font-size:13px;font-weight:800;color:{r['color']}">{r['comp']}</span></td>
              <td style="padding:8px 8px;text-align:center;color:#64748b">{r['arrow']}</td>
              <td style="padding:8px 8px;text-align:center">
                <span style="background:{r['color']};color:white;padding:3px 7px;border-radius:10px;font-size:10px;font-weight:700;white-space:nowrap">{r['sig']} {r['label']}</span></td>
            </tr>"""

    return f"""
  <div class="stitle">🧠 Fundamental Trend Analysis</div>
  <table>
    <thead><tr>
      <th style="text-align:left;padding-left:12px">Stock</th>
      <th style="text-align:right">Price</th>
      <th>Fund<br><span style="font-weight:400;font-size:9px">P·V·H·G /100</span></th>
      <th>Tech<br><span style="font-weight:400;font-size:9px">/100</span></th>
      <th>Score<br><span style="font-weight:400;font-size:9px">/100</span></th>
      <th>Trend</th>
      <th>Signal</th>
    </tr></thead>
    <tbody>{rows_html}</tbody>
  </table>
  <div class="note">
    <b>Composite = Fundamental×60% + Technical×40%</b> | Pillars: <b>P</b>=Profitability <b>V</b>=Valuation <b>H</b>=Health <b>G</b>=Growth<br>
    ✅✅✅ Must Buy (85+) · ✅✅ Strong Buy (70+) · ✅ Buy (55+) · ⚪ Hold (40+) · ❌ Sell (25+) · ❌❌ Strong Sell (10+)
  </div>"""


def build_price_perf_section(fta_rows):
    """Separate price performance table: 1W / 1M / 3M / 1Y for all FTA tickers."""
    import yfinance as yf
    from datetime import datetime, timedelta, timezone

    group_order = [
        ("🇺🇸 US Stocks", lambda t: t not in {"Z74.SI","C6L.SI","HST.SI","D05.SI","VOO","QQQ","SPY","SPMO","IBIT","ITC.NS","RELIANCE.NS","TMCV.NS","^NSEI","GC=F"}),
        ("📊 US ETFs",    lambda t: t in {"VOO","QQQ","SPY","SPMO","IBIT"}),
        ("🇸🇬 Singapore", lambda t: t in {"Z74.SI","C6L.SI","HST.SI","D05.SI"}),
        ("🇮🇳 India",     lambda t: t in {"ITC.NS","RELIANCE.NS","TMCV.NS","^NSEI","GC=F"}),
    ]

    def pct(now, then):
        if not then or then == 0: return None
        return (now - then) / then * 100

    def cell(v):
        if v is None: return '<td style="text-align:right;padding:7px 6px;color:#94a3b8;font-size:11px">—</td>'
        color = "#22c55e" if v >= 0 else "#ef4444"
        sign = "+" if v >= 0 else ""
        return f'<td style="text-align:right;padding:7px 6px;font-weight:600;color:{color};font-size:12px">{sign}{v:.1f}%</td>'

    rows_html = ""
    for group_label, filter_fn in group_order:
        group_tickers = [r for r in fta_rows if filter_fn(r["ticker"])]
        if not group_tickers: continue
        rows_html += f'<tr style="background:#eef2ff"><td colspan="6" style="padding:6px 12px;font-weight:700;color:#0c2461;font-size:11px;text-transform:uppercase;letter-spacing:.5px">{group_label}</td></tr>'
        for r in group_tickers:
            t = r["ticker"]
            try:
                hist = yf.Ticker(t).history(period="1y")["Close"]
                if hist.empty: continue
                now_p = float(hist.iloc[-1])
                w1  = float(hist.iloc[-6])  if len(hist) >= 6  else None
                m1  = float(hist.iloc[-22]) if len(hist) >= 22 else None
                m3  = float(hist.iloc[-66]) if len(hist) >= 66 else None
                y1  = float(hist.iloc[0])
                rows_html += f"""<tr style="border-bottom:1px solid #f0f0f0">
                  <td style="padding:7px 12px;font-weight:700;color:#1e293b;font-size:12px">{t}<br>
                    <span style="font-size:10px;font-weight:400;color:#94a3b8">{r["name"]}</span></td>
                  <td style="text-align:right;padding:7px 8px;font-size:12px;font-weight:600;color:#1e293b">{r["price"]:,.2f}</td>
                  {cell(pct(now_p,w1))}{cell(pct(now_p,m1))}{cell(pct(now_p,m3))}{cell(pct(now_p,y1))}
                </tr>"""
            except: continue

    return f"""
  <div class="stitle">📈 Price Performance</div>
  <table>
    <thead><tr>
      <th style="text-align:left;padding-left:12px">Stock / ETF</th>
      <th style="text-align:right">Price</th>
      <th style="text-align:right">1W</th>
      <th style="text-align:right">1M</th>
      <th style="text-align:right">3M</th>
      <th style="text-align:right">1Y</th>
    </tr></thead>
    <tbody>{rows_html}</tbody>
  </table>
  <div class="note">Price % change vs close N trading days ago. All prices from Yahoo Finance.</div>"""


INDIA_MF_SCHEMES = [
    {"code": "122639", "name": "Parag Parikh Flexi Cap",        "units": None},
    {"code": "118834", "name": "Mirae Asset Large & Midcap",    "units": None},
    {"code": "120505", "name": "Axis Midcap",                   "units": None},
    {"code": "125354", "name": "Axis Small Cap",                "units": None},
]

def fetch_india_mf_navs():
    import requests as req
    results = []
    for mf in INDIA_MF_SCHEMES:
        try:
            r = req.get(f"https://api.mfapi.in/mf/{mf['code']}", timeout=10)
            d = r.json()
            data = d.get("data", [])
            if not data: continue
            nav_today  = float(data[0]["nav"])
            nav_1w     = float(data[5]["nav"])  if len(data) > 5  else None
            nav_1m     = float(data[21]["nav"]) if len(data) > 21 else None
            nav_3m     = float(data[63]["nav"]) if len(data) > 63 else None
            nav_1y     = float(data[252]["nav"])if len(data) > 252 else None
            date_str   = data[0]["date"]
            def chg(now, then): return (now-then)/then*100 if then else None
            results.append({
                "name": mf["name"], "nav": nav_today, "date": date_str,
                "units": mf["units"],
                "chg_1w": chg(nav_today, nav_1w),
                "chg_1m": chg(nav_today, nav_1m),
                "chg_3m": chg(nav_today, nav_3m),
                "chg_1y": chg(nav_today, nav_1y),
            })
        except: continue
    return results

def build_india_mf_section(mf_data, usd_inr):
    def cell(v):
        if v is None: return '<td style="text-align:right;padding:7px 6px;color:#94a3b8;font-size:11px">—</td>'
        color = "#22c55e" if v >= 0 else "#ef4444"
        sign = "+" if v >= 0 else ""
        return f'<td style="text-align:right;padding:7px 6px;font-weight:600;color:{color};font-size:12px">{sign}{v:.1f}%</td>'

    rows = ""
    for mf in mf_data:
        val_str = "—"
        if mf["units"]:
            val_inr = mf["nav"] * mf["units"]
            val_str = f"<b>₹{val_inr:,.0f}</b>"
        rows += f"""<tr style="border-bottom:1px solid #f0f0f0">
          <td style="padding:8px 12px;font-weight:600;color:#1e293b">{mf["name"]}<br>
            <span style="font-size:10px;color:#94a3b8">NAV as of {mf["date"]}</span></td>
          <td style="text-align:right;padding:8px 8px;font-weight:700;color:#0c2461;font-size:13px">₹{mf["nav"]:,.2f}</td>
          {cell(mf["chg_1w"])}{cell(mf["chg_1m"])}{cell(mf["chg_3m"])}{cell(mf["chg_1y"])}
          <td style="text-align:right;padding:8px 8px;font-size:12px">{val_str}</td>
        </tr>"""

    return f"""
  <div class="stitle">🇮🇳 India Mutual Funds (Live NAV via mfapi.in)</div>
  <table>
    <thead><tr>
      <th style="text-align:left;padding-left:12px">Fund</th>
      <th style="text-align:right">NAV (₹)</th>
      <th style="text-align:right">1W</th>
      <th style="text-align:right">1M</th>
      <th style="text-align:right">3M</th>
      <th style="text-align:right">1Y</th>
      <th style="text-align:right">Value</th>
    </tr></thead>
    <tbody>{rows}</tbody>
  </table>
  <div class="note">Live NAV from AMFI via mfapi.in · Share your unit holdings to see portfolio value</div>"""


def build_portfolio_section(portfolio, mkt_data, usd_inr, usd_sgd):
    rows = ""
    total_usd = 0
    regions = ["USA", "India", "Singapore"]
    region_flags = {"USA": "🇺🇸", "India": "🇮🇳", "Singapore": "🇸🇬"}

    for region in regions:
        region_holdings = [h for h in portfolio if h["region"] == region]
        if not region_holdings:
            continue
        flag = region_flags[region]
        rows += f'<tr style="background:#eef2ff"><td colspan="6" style="padding:7px 12px;font-weight:700;color:#0c2461;font-size:12px;letter-spacing:.3px">{flag} {region}</td></tr>'

        for h in region_holdings:
            ticker = h.get("ticker")
            name = h["name"]
            ptype = h.get("type","").replace("_"," ").title()

            if ticker and ticker != "GC=F" and mkt_data.get(ticker):
                d = mkt_data[ticker]
                price = d["price"]
                qty = h.get("quantity", 0)
                avg = h.get("avg_price", price)
                value_usd = price * qty
                gl_pct = (price - avg) / avg * 100 if avg else 0
                gl_color = "#22c55e" if gl_pct >= 0 else "#ef4444"
                chg_color = "#22c55e" if d["chg"] >= 0 else "#ef4444"
                chg_sign = "+" if d["chg"] >= 0 else ""
                gl_sign = "+" if gl_pct >= 0 else ""
                if region == "USA":
                    val_str = f"<b>${value_usd:,.0f}</b>"
                elif region == "Singapore":
                    val_str = f"<b>S${value_usd * usd_sgd:,.0f}</b>"
                else:
                    val_str = f"<b>₹{value_usd * usd_inr:,.0f}</b>"
                total_usd += value_usd
                rows += f'''<tr style="border-bottom:1px solid #f0f0f0">
                  <td style="padding:8px 12px;font-weight:600;color:#1e293b">{name}</td>
                  <td style="padding:8px 8px;text-align:center;font-size:11px;color:#64748b">{ptype}</td>
                  <td style="padding:8px 8px;text-align:right;font-size:12px">${price:.2f}<br>
                    <span style="color:{chg_color};font-size:10px">{chg_sign}{d["chg"]:.1f}% today</span></td>
                  <td style="padding:8px 8px;text-align:right;font-size:12px;color:#64748b">{qty:.3f} units</td>
                  <td style="padding:8px 8px;text-align:right">{val_str}</td>
                  <td style="padding:8px 8px;text-align:right;font-size:11px;color:{gl_color};font-weight:700">{gl_sign}{gl_pct:.1f}%<br>
                    <span style="font-size:9px;color:#94a3b8">avg ${avg:.2f}</span></td>
                </tr>'''

            elif ticker == "GC=F" and mkt_data.get("GC=F"):
                gold_price_usd = mkt_data["GC=F"]["price"]
                gold_chg = mkt_data["GC=F"]["chg"]
                qty_grams = h.get("quantity_grams", 0)
                value_usd = (gold_price_usd / 31.1035) * qty_grams
                value_inr = value_usd * usd_inr
                total_usd += value_usd
                chg_color = "#22c55e" if gold_chg >= 0 else "#ef4444"
                chg_sign = "+" if gold_chg >= 0 else ""
                rows += f'''<tr style="border-bottom:1px solid #f0f0f0">
                  <td style="padding:8px 12px;font-weight:600;color:#1e293b">{name}</td>
                  <td style="padding:8px 8px;text-align:center;font-size:11px;color:#64748b">Gold Bond</td>
                  <td style="padding:8px 8px;text-align:right;font-size:12px">${gold_price_usd:,.0f}/oz<br>
                    <span style="color:{chg_color};font-size:10px">{chg_sign}{gold_chg:.1f}% today</span></td>
                  <td style="padding:8px 8px;text-align:right;font-size:12px;color:#64748b">{qty_grams}g</td>
                  <td style="padding:8px 8px;text-align:right"><b>₹{value_inr:,.0f}</b></td>
                  <td style="padding:8px 8px;text-align:right;font-size:11px;color:#22c55e;font-weight:700">Live ✓</td>
                </tr>'''

            elif h.get("value_inr"):
                value_usd = h["value_inr"] / usd_inr
                total_usd += value_usd
                rows += f'''<tr style="border-bottom:1px solid #f0f0f0">
                  <td style="padding:8px 12px;font-weight:600;color:#1e293b">{name}</td>
                  <td style="padding:8px 8px;text-align:center;font-size:11px;color:#64748b">{ptype}</td>
                  <td style="padding:8px 8px;text-align:right;font-size:11px;color:#94a3b8" colspan="2">Last saved NAV</td>
                  <td style="padding:8px 8px;text-align:right"><b>₹{h["value_inr"]:,.0f}</b></td>
                  <td style="padding:8px 8px;text-align:right;font-size:10px;color:#f59e0b;font-weight:600">⚠ stale</td>
                </tr>'''

            elif h.get("value_sgd"):
                value_usd = h["value_sgd"] / usd_sgd
                total_usd += value_usd
                rows += f'''<tr style="border-bottom:1px solid #f0f0f0">
                  <td style="padding:8px 12px;font-weight:600;color:#1e293b">{name}</td>
                  <td style="padding:8px 8px;text-align:center;font-size:11px;color:#64748b">{ptype}</td>
                  <td style="padding:8px 8px;text-align:right;font-size:11px;color:#94a3b8" colspan="2">SGD Cash/MM</td>
                  <td style="padding:8px 8px;text-align:right"><b>S${h["value_sgd"]:,.0f}</b></td>
                  <td style="padding:8px 8px;text-align:right;font-size:10px;color:#94a3b8">—</td>
                </tr>'''

    return f"""
  <div class="stitle">💼 My Portfolio</div>
  <table>
    <thead><tr>
      <th style="text-align:left;padding-left:12px">Holding</th>
      <th>Type</th>
      <th style="text-align:right">Live Price</th>
      <th style="text-align:right">Qty</th>
      <th style="text-align:right">Value</th>
      <th style="text-align:right">G/L</th>
    </tr></thead>
    <tbody>{rows}</tbody>
  </table>
  <div style="padding:10px 18px;background:#f0fdf4;border-top:2px solid #22c55e;text-align:right;font-size:13px">
    <span style="color:#64748b">Tracked Total: </span>
    <span style="font-weight:800;color:#166534;font-size:16px">${total_usd:,.0f} USD</span>
    &nbsp;<span style="font-size:11px;color:#94a3b8">(India MF NAVs are last saved — refresh Kuvera for live)</span>
  </div>"""

def build_market_section(indices_data, comm_data, crypto_data, forex_data, us_movers, sg_movers):
    def mkt_rows(data_dict, symbol_map):
        rows = ""
        for sym, name in symbol_map.items():
            d = data_dict.get(sym)
            if not d: continue
            price_str = fmt_price(d['price'], sym)
            rows += f"""<tr class="mkt-row">
              <td>{name}</td>
              <td style="text-align:right;font-weight:600">{price_str}</td>
              <td style="text-align:right">{fmt_chg(d['chg'])}</td>
              <td style="text-align:right">{fmt_chg(d.get('week_chg'))}</td>
            </tr>"""
        return rows

    def movers_table(movers_dict, label_map):
        rows_g = ""
        rows_l = ""
        all_m = [(s, d) for s, d in movers_dict.items() if d]
        gainers = sorted([(s,d) for s,d in all_m if d['chg']>0], key=lambda x: -x[1]['chg'])[:4]
        losers  = sorted([(s,d) for s,d in all_m if d['chg']<0], key=lambda x:  x[1]['chg'])[:4]
        flag = "🇸🇬" if any(".SI" in s for s,_ in all_m) else "🇺🇸"
        for s, d in gainers:
            name = label_map.get(s, s.replace('.SI',''))
            rows_g += f'<tr><td style="padding:4px 8px;font-weight:600">{flag} {name}</td><td style="padding:4px 8px;text-align:right;color:#22c55e;font-weight:700">+{d["chg"]:.1f}%</td></tr>'
        for s, d in losers:
            name = label_map.get(s, s.replace('.SI',''))
            rows_l += f'<tr><td style="padding:4px 8px;font-weight:600">{flag} {name}</td><td style="padding:4px 8px;text-align:right;color:#ef4444;font-weight:700">{d["chg"]:.1f}%</td></tr>'
        return rows_g, rows_l

    us_g, us_l = movers_table(us_movers, US_NAMES)
    sg_g, sg_l = movers_table(sg_movers, SG_NAMES)

    return f"""
  <div class="stitle">🌍 Market Overview</div>
  <table>
    <thead><tr>
      <th style="text-align:left;padding-left:12px">Index</th>
      <th style="text-align:right">Price</th>
      <th style="text-align:right">1D</th>
      <th style="text-align:right">1W</th>
    </tr></thead>
    <tbody>{mkt_rows(indices_data, INDICES)}</tbody>
  </table>
  <table style="margin-top:1px">
    <thead><tr>
      <th style="text-align:left;padding-left:12px">Commodity / Crypto</th>
      <th style="text-align:right">Price</th><th style="text-align:right">1D</th><th style="text-align:right">1W</th>
    </tr></thead>
    <tbody>
      {mkt_rows(comm_data, COMMODITIES)}
      {mkt_rows(crypto_data, CRYPTO)}
    </tbody>
  </table>
  <table style="margin-top:1px">
    <thead><tr>
      <th style="text-align:left;padding-left:12px">Currency</th>
      <th style="text-align:right">Rate</th><th style="text-align:right">1D</th><th style="text-align:right">1W</th>
    </tr></thead>
    <tbody>{mkt_rows(forex_data, FOREX)}</tbody>
  </table>
  <table style="margin-top:1px;background:#fafafa">
    <thead><tr>
      <th colspan="2" style="text-align:left;padding-left:12px">📈 Top Gainers</th>
      <th colspan="2" style="text-align:left;padding-left:16px">📉 Top Losers</th>
    </tr></thead>
    <tbody>
      <tr>
        <td colspan="2" style="padding:0;vertical-align:top"><table style="width:100%">{us_g}{sg_g}</table></td>
        <td colspan="2" style="padding:0;vertical-align:top"><table style="width:100%">{us_l}{sg_l}</table></td>
      </tr>
    </tbody>
  </table>"""

def build_news_section(articles):
    items = ""
    for i, a in enumerate(articles[:15]):
        bg = "#fff" if i % 2 == 0 else "#f8fafc"
        src_color = "#2563eb"
        pub = ""
        if a.get('published'):
            delta = datetime.now(timezone.utc) - a['published']
            hrs = int(delta.total_seconds() / 3600)
            pub = f" · {hrs}h ago" if hrs < 24 else ""
        items += f"""<tr style="background:{bg};border-bottom:1px solid #f0f0f0">
          <td style="padding:9px 12px">
            <a href="{a['link']}" style="color:#1e293b;text-decoration:none;font-weight:600;font-size:13px">{a['title']}</a><br>
            <span style="font-size:10px;color:{src_color};font-weight:600">{a['source']}</span><span style="font-size:10px;color:#94a3b8">{pub}</span>
          </td>
          <td style="padding:9px 8px;text-align:right;white-space:nowrap">
            <span style="background:#f0f4ff;color:#2563eb;padding:2px 7px;border-radius:8px;font-size:10px;font-weight:700">{a['score']}</span>
          </td>
        </tr>"""
    return f"""
  <div class="stitle">📡 Tech & Market Intel</div>
  <table>
    <thead><tr>
      <th style="text-align:left;padding-left:12px">Story</th>
      <th style="text-align:right;padding-right:8px">Score</th>
    </tr></thead>
    <tbody>{items}</tbody>
  </table>"""

# ── Main ───────────────────────────────────────────────────────────────────────
def main():
    print(f"[{TIME_STR}] Starting Daily Digest...")

    # 1. FTA
    print("  → Fetching FTA data...")
    fta_rows = []
    for t in FTA_TICKERS:
        try:
            tk = yf.Ticker(t)
            info = tk.info
            hist = tk.history(period='1y')
            closes = list(hist['Close'])
            price = info.get('regularMarketPrice') or (closes[-1] if closes else None)
            if not price:
                print(f"     {t}: skipped (no price data)")
                continue
            chg = info.get('regularMarketChangePercent') or 0
            p, v, h, g, fund = calc_fundamental(info)
            ma, mo, pa, vol, tech = calc_technical(closes, price)
            comp = round(fund * 0.60 + tech * 0.40)
            sig, label, color, bg = signal(comp)
            arrow = trend_arrow(tech)
            name = FTA_NAMES.get(t, info.get('shortName', '')[:22])
            fta_rows.append({'ticker': t, 'name': name, 'price': price, 'chg': chg,
                             'fund': fund, 'tech': tech, 'comp': comp,
                             'sig': sig, 'label': label, 'arrow': arrow,
                             'color': color, 'bg': bg, 'p': p, 'v': v, 'h': h, 'g': g})
            print(f"     {t}: fund={fund} tech={tech} comp={comp} → {label}")
        except Exception as e:
            print(f"     {t}: skipped ({type(e).__name__})")

    # 2. Market data
    print("  → Fetching market data...")
    portfolio_extra = ["VOO", "QQQ", "GC=F"]
    all_syms = list(INDICES.keys()) + list(COMMODITIES.keys()) + list(CRYPTO.keys()) + list(FOREX.keys()) + US_MOVERS + SG_MOVERS + portfolio_extra
    mkt_data = {}
    for sym in all_syms:
        d = fetch_quote(sym)
        mkt_data[sym] = d
    indices_data = {s: mkt_data[s] for s in INDICES}
    comm_data = {s: mkt_data[s] for s in COMMODITIES}
    crypto_data = {s: mkt_data[s] for s in CRYPTO}
    forex_data = {s: mkt_data[s] for s in FOREX}
    us_movers_data = {s: mkt_data[s] for s in US_MOVERS}
    sg_movers_data = {s: mkt_data[s] for s in SG_MOVERS}

    # 3. News
    print("  → Fetching news...")
    articles = fetch_news()
    print(f"     {len(articles)} stories scored")

    # Get live USD/INR and USD/SGD
    usd_inr = mkt_data.get("INR=X", {}).get('price', 86.0) if mkt_data.get("INR=X") else 86.0
    usd_sgd = mkt_data.get("SGD=X", {}).get('price', 1.274) if mkt_data.get("SGD=X") else 1.274

    # Build HTML
    fta_html = build_fta_section(fta_rows)
    perf_html = build_price_perf_section(fta_rows)

    print("  \u2192 Fetching India MF NAVs...")
    india_mf_data = fetch_india_mf_navs()
    print(f"     {len(india_mf_data)} funds loaded")
    india_mf_html = build_india_mf_section(india_mf_data, usd_inr)

    mkt_html = build_market_section(indices_data, comm_data, crypto_data, forex_data, us_movers_data, sg_movers_data)
    news_html = build_news_section(articles)

    html = f"""<!DOCTYPE html><html><head><meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
body{{font-family:'Segoe UI',Arial,sans-serif;background:#f1f5f9;margin:0;padding:16px}}
.wrap{{background:white;max-width:740px;margin:0 auto;border-radius:14px;box-shadow:0 4px 20px rgba(0,0,0,.10);overflow:hidden}}
.hdr{{background:linear-gradient(135deg,#0c2461 0%,#1e3799 60%,#0a3d62 100%);color:white;padding:28px;text-align:center}}
.hdr h1{{margin:0 0 4px;font-size:24px;letter-spacing:-.5px}}
.hdr p{{margin:0;font-size:13px;opacity:.8}}
.stitle{{background:#f8f9fa;padding:11px 18px;font-size:13px;font-weight:700;color:#0c2461;border-bottom:2px solid #1e3799;border-top:1px solid #eee;letter-spacing:.3px;text-transform:uppercase}}
table{{width:100%;border-collapse:collapse}}
th{{background:#f0f2f5;padding:8px 8px;text-align:center;font-weight:600;color:#64748b;font-size:10px;text-transform:uppercase;letter-spacing:.5px}}
th:first-child{{text-align:left;padding-left:12px}}
.mkt-row td{{padding:8px 12px;font-size:13px;border-bottom:1px solid #f0f0f0}}
.mover-item{{font-size:12px;padding:2px 0}}
.note{{padding:11px 18px;font-size:11px;color:#64748b;background:#f8fafc;border-left:3px solid #2563eb}}
.footer{{background:#f8f9fa;padding:14px 20px;font-size:11px;color:#9ca3af;text-align:center;border-top:1px solid #eee}}
a{{color:#2563eb}}
</style></head><body>
<div class="wrap">
  <div class="hdr">
    <h1>📊 Daily Digest</h1>
    <p>{DATE_STR} · {TIME_STR} · Fundamental · Markets · Tech Intel</p>
  </div>
  {fta_html}
  {perf_html}
  {india_mf_html}
  {mkt_html}
  {news_html}
  <div class="footer">
    Personal Agent · {DATE_STR} · Data: Yahoo Finance + RSS Feeds<br>
    Not financial advice. Piotroski F-Score + Altman Z-Score framework.
  </div>
</div></body></html>"""

    # Send
    print("  → Sending email...")
    msg = MIMEMultipart('alternative')
    msg['Subject'] = f'📊 Daily Digest — {DATE_STR}'
    msg['From'] = SMTP_USER
    msg['To'] = ', '.join(RECIPIENTS)
    msg.attach(MIMEText(html, 'html'))
    server = smtplib.SMTP_SSL(SMTP_HOST, SMTP_PORT)
    server.login(SMTP_USER, SMTP_PASS)
    server.sendmail(SMTP_USER, RECIPIENTS, msg.as_string())
    server.quit()
    print(f"✅ Daily Digest sent to {RECIPIENTS}")
    print(f"   FTA: {len(fta_rows)} stocks | News: {len(articles)} stories")

if __name__ == "__main__":
    main()
