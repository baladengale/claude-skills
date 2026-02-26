---
name: tech-intel
description: Daily Market & Tech Intelligence newsletter - concurrent RSS aggregation, keyword scoring, and curated top stories presentation.
metadata:
  emoji: "📡"
  requires:
    bins: ["curl", "bash"]
---

# Tech Intel — Daily Market & Tech Pulse

Plain-language guide for aggregating and scoring tech/market news from RSS feeds. Claude fetches articles from 12 sources, scores them by keyword relevance, and presents the top stories as a curated digest.

## When to Activate

Activate when the user asks about:
- Tech news, tech intelligence, market intelligence
- Daily newsletter, tech newsletter, market newsletter
- Send tech digest, send market digest
- RSS feeds, news aggregation
- Tech pulse, market pulse

---

## RSS Feed Sources

Fetch from all 12 sources concurrently:

| Source | URL | Category |
|--------|-----|----------|
| TechCrunch | `https://techcrunch.com/feed/` | Tech |
| Ars Technica | `https://feeds.arstechnica.com/arstechnica/index` | Tech |
| The Verge | `https://www.theverge.com/rss/index.xml` | Tech |
| Hacker News | `https://hnrss.org/newest?points=100` | Tech |
| Wired | `https://www.wired.com/feed/rss` | Tech |
| CNBC Top News | `https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=100003114` | Markets |
| CNBC World | `https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=100727362` | Markets |
| MarketWatch | `https://feeds.marketwatch.com/marketwatch/topstories/` | Markets |
| Yahoo Finance | `https://finance.yahoo.com/news/rssindex` | Markets |
| BBC Business | `https://feeds.bbci.co.uk/news/business/rss.xml` | Business |
| NPR Business | `https://feeds.npr.org/1006/rss.xml` | Business |
| Reuters | `https://www.reutersagency.com/feed/?taxonomy=best-sectors&post_type=best` | Markets |

### Fetch a Single Feed

```bash
curl -s "https://techcrunch.com/feed/" | \
  python3 -c "
import sys, xml.etree.ElementTree as ET
root = ET.fromstring(sys.stdin.read())
for item in root.findall('.//item')[:5]:
    title = item.findtext('title', '').strip()
    link = item.findtext('link', '').strip()
    pub = item.findtext('pubDate', '').strip()
    print(f'{pub[:16]}  {title[:80]}')"
```

---

## Processing Pipeline

Work through these steps in order:

### Step 1 — Filter to Last 24 Hours

Only include articles published within the last 24 hours. Parse `<pubDate>` (RSS) or `<updated>` (Atom) fields.

Common date formats:
- `Mon, 15 Jan 2024 10:23:01 +0000` (RFC 1123)
- `2024-01-15T10:23:01Z` (RFC 3339)
- `2024-01-15T10:23:01-07:00`

### Step 2 — Deduplicate by URL

Remove articles with the same URL (normalized: strip query params and trailing slashes, lowercase).

### Step 3 — Score Each Article

Calculate a relevance score using keyword matching in title + description:

| Keyword | Score |
|---------|-------|
| market disruption | 10 |
| revolutionary | 9 |
| acquisition | 8 |
| semiconductor | 8 |
| fintech | 7 |
| ai, artificial intelligence | 7 |
| ipo | 6 |
| data breach | 6 |
| merger | 6 |
| regulation | 5 |
| earnings, quarterly results | 5 |
| cybersecurity | 5 |
| quantum | 5 |
| startup | 4 |
| funding | 4 |
| blockchain | 4 |

**Scoring rules:**
- Add base score for each keyword found in title OR description
- **Double the score** if keyword is in the title (title matches get bonus weight)
- Add **+3 recency bonus** for articles published in the last 6 hours
- Add **+2 category bonus** for articles from Markets category sources

### Step 4 — Select Top Stories

Sort by score descending, then by recency descending. Select the top 25 articles.

---

## Output Format

Present as a numbered digest:

```
=== Daily Market & Tech Pulse ===
Date: Monday, 15 Jan 2024
Sources: 12 | Articles scanned: 342 | Top articles: 25

 1. [Score:23] NVIDIA Acquires AI Startup for $2.1B in Semiconductor Push
    TechCrunch | Markets | 10:45 SGT
    https://techcrunch.com/...
    Keywords: acquisition, semiconductor, ai

 2. [Score:18] Fed Chair Signals Rate Cuts Amid Inflation Data Surprise
    CNBC Top News | Markets | 09:12 SGT
    https://cnbc.com/...
    Keywords: regulation, earnings

 3. [Score:15] OpenAI Raises $6B in Latest Funding Round
    The Verge | Tech | 08:30 SGT
    https://theverge.com/...
    Keywords: ai, funding

...
```

---

## Keyword Scoring Logic (Detailed)

```python
# Pseudocode for scoring one article
def score_article(title, description):
    text = (title + " " + description).lower()
    title_lower = title.lower()
    score = 0
    matched = []

    for keyword, points in keyword_scores.items():
        if keyword in text:
            score += points           # base score
            matched.append(keyword)
            if keyword in title_lower:
                score += points       # title bonus (doubles the score)

    # Recency bonus
    if hours_since_published < 6:
        score += 3

    # Category bonus
    if source_category == "Markets":
        score += 2

    return score, matched
```

---

## Interpreting Results

**High-score articles (>15):** Multiple high-value keywords or a title match on a major keyword — these are the day's most significant stories.

**Medium-score articles (8–14):** Relevant but less impactful. Worth a quick skim.

**Low-score articles (<8):** Background noise. Include only if filling out the top 25.

**What to highlight in the summary:**
- Any acquisition or IPO news (market-moving)
- AI/semiconductor developments (sector trends)
- Regulatory changes (policy impact)
- Data breaches or cybersecurity incidents (risk)
- Earnings surprises (market reaction)

---

## References

- [TechCrunch RSS](https://techcrunch.com/feed/)
- [Hacker News RSS (points>100)](https://hnrss.org/newest?points=100)
- [CNBC RSS Feeds](https://www.cnbc.com/rss-feeds/)
- [Yahoo Finance RSS](https://finance.yahoo.com/news/rssindex)
