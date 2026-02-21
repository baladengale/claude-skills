package main

import (
	"bytes"
	_ "embed"
	"encoding/xml"
	"fmt"
	"html"
	"html/template"
	"io"
	"log"
	"net/smtp"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"net/http"
)

// ─── Data Models ─────────────────────────────────────────────────────────────

type FeedSource struct {
	Name     string
	URL      string
	Category string
}

type Article struct {
	Title       string
	Link        string
	Description string
	Summary     string
	Source      string
	Category    string
	PublishedAt time.Time
	Score       int
	Keywords    []string
}

type FetchResult struct {
	Articles []Article
	Source   string
	Err      error
}

type Newsletter struct {
	Title      string
	Date       string
	Articles   []Article
	Sources    []string
	Generated  string
	TotalFound int
}

type EmailConfig struct {
	SMTPHost    string
	SMTPPort    int
	Username    string
	AppPassword string
	FromAddr    string
	Recipients  []string
}

// ─── RSS/Atom XML Structs ────────────────────────────────────────────────────

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title   string     `xml:"title"`
	Links   []atomLink `xml:"link"`
	Summary string     `xml:"summary"`
	Content string     `xml:"content"`
	Updated string     `xml:"updated"`
	ID      string     `xml:"id"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

// ─── Configuration ───────────────────────────────────────────────────────────

var sgt = time.FixedZone("SGT", 8*60*60)

var defaultFeeds = []FeedSource{
	// Tech
	{Name: "TechCrunch", URL: "https://techcrunch.com/feed/", Category: "Tech"},
	{Name: "Ars Technica", URL: "https://feeds.arstechnica.com/arstechnica/index", Category: "Tech"},
	{Name: "The Verge", URL: "https://www.theverge.com/rss/index.xml", Category: "Tech"},
	{Name: "Hacker News", URL: "https://hnrss.org/newest?points=100", Category: "Tech"},
	{Name: "Wired", URL: "https://www.wired.com/feed/rss", Category: "Tech"},

	// Markets & Finance
	{Name: "CNBC Top News", URL: "https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=100003114", Category: "Markets"},
	{Name: "CNBC World", URL: "https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=100727362", Category: "Markets"},
	{Name: "MarketWatch", URL: "https://feeds.marketwatch.com/marketwatch/topstories/", Category: "Markets"},
	{Name: "Yahoo Finance", URL: "https://finance.yahoo.com/news/rssindex", Category: "Markets"},

	// Business / Geopolitics
	{Name: "BBC Business", URL: "https://feeds.bbci.co.uk/news/business/rss.xml", Category: "Business"},
	{Name: "NPR Business", URL: "https://feeds.npr.org/1006/rss.xml", Category: "Business"},
	{Name: "Reuters", URL: "https://www.reutersagency.com/feed/?taxonomy=best-sectors&post_type=best", Category: "Markets"},
}

var keywordScores = map[string]int{
	"market disruption":       10,
	"revolutionary":           9,
	"acquisition":             8,
	"semiconductor":           8,
	"fintech":                 7,
	"ai":                      7,
	"artificial intelligence": 7,
	"ipo":                     6,
	"merger":                  6,
	"regulation":              5,
	"earnings":                5,
	"quarterly results":       5,
	"cybersecurity":           5,
	"quantum":                 5,
	"data breach":             6,
	"startup":                 4,
	"funding":                 4,
	"blockchain":              4,
}

//go:embed template.html
var embeddedTemplate string

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

// ─── Main ────────────────────────────────────────────────────────────────────

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	loadEnvFile()

	cfg := EmailConfig{
		SMTPHost:    "smtp.gmail.com",
		SMTPPort:    587,
		Username:    envOrDefault("GMAIL_USER", "SMTP_USER_PLACEHOLDER"),
		AppPassword: os.Getenv("GMAIL_APP_PASSWORD"),
		FromAddr:    envOrDefault("GMAIL_USER", "SMTP_USER_PLACEHOLDER"),
		Recipients:  parseRecipients(envOrDefault("NEWSLETTER_RECIPIENTS", "SMTP_USER_PLACEHOLDER,badengal@visa.com")),
	}

	if cfg.AppPassword == "" {
		log.Println("WARNING: GMAIL_APP_PASSWORD not set; email delivery will be skipped")
	}

	// Concurrent RSS fetch
	feeds := defaultFeeds
	ch := make(chan FetchResult, len(feeds))
	var wg sync.WaitGroup

	for _, feed := range feeds {
		wg.Add(1)
		go fetchFeed(feed, ch, &wg)
	}
	go func() { wg.Wait(); close(ch) }()

	// Collect results
	var allArticles []Article
	var sources []string
	for result := range ch {
		if result.Err != nil {
			log.Printf("WARN: feed %q failed: %v", result.Source, result.Err)
			continue
		}
		allArticles = append(allArticles, result.Articles...)
		sources = append(sources, result.Source)
		log.Printf("OK: %s -> %d articles", result.Source, len(result.Articles))
	}

	totalFound := len(allArticles)
	log.Printf("Total articles fetched: %d from %d sources", totalFound, len(sources))

	// Filter to last 24 hours
	cutoff := time.Now().Add(-24 * time.Hour)
	allArticles = filterByTime(allArticles, cutoff)
	log.Printf("After 24h filter: %d articles", len(allArticles))

	// Deduplicate
	allArticles = deduplicateByURL(allArticles)
	log.Printf("After dedup: %d articles", len(allArticles))

	// Score
	for i := range allArticles {
		scoreArticle(&allArticles[i])
	}

	// Sort: score DESC, then time DESC
	sort.Slice(allArticles, func(i, j int) bool {
		if allArticles[i].Score != allArticles[j].Score {
			return allArticles[i].Score > allArticles[j].Score
		}
		return allArticles[i].PublishedAt.After(allArticles[j].PublishedAt)
	})

	// Top 25
	if len(allArticles) > 25 {
		allArticles = allArticles[:25]
	}

	// Build newsletter
	sort.Strings(sources)
	now := time.Now().In(sgt)
	newsletter := Newsletter{
		Title:      "Daily Market & Tech Pulse",
		Date:       now.Format("Monday, 02 Jan 2006"),
		Articles:   allArticles,
		Sources:    sources,
		Generated:  now.Format("15:04:05 SGT, 02 Jan 2006"),
		TotalFound: totalFound,
	}

	// Render HTML
	htmlBody, err := renderTemplate(newsletter)
	if err != nil {
		log.Fatalf("Template rendering failed: %v", err)
	}

	// Send email
	if cfg.AppPassword != "" {
		subject := fmt.Sprintf("Daily Market & Tech Pulse - %s", now.Format("02 Jan 2006"))
		if err := sendEmail(cfg, htmlBody, subject); err != nil {
			log.Printf("Email send failed: %v", err)
			saveFallback(htmlBody, now)
		} else {
			log.Printf("Email sent successfully to %v", cfg.Recipients)
		}
	} else {
		saveFallback(htmlBody, now)
	}

	// Console summary
	fmt.Printf("\n=== %s ===\n", newsletter.Title)
	fmt.Printf("Date: %s\n", newsletter.Date)
	fmt.Printf("Sources: %d | Articles scanned: %d | Top articles: %d\n\n",
		len(sources), totalFound, len(allArticles))
	for i, a := range allArticles {
		fmt.Printf("%d. [Score:%d] %s\n   %s | %s\n   %s\n\n",
			i+1, a.Score, a.Title, a.Source,
			a.PublishedAt.In(sgt).Format("15:04 SGT"),
			a.Link)
	}
}

// ─── RSS Fetching ────────────────────────────────────────────────────────────

func fetchFeed(source FeedSource, ch chan<- FetchResult, wg *sync.WaitGroup) {
	defer wg.Done()

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", source.URL, nil)
	if err != nil {
		ch <- FetchResult{Source: source.Name, Err: err}
		return
	}
	req.Header.Set("User-Agent", "TechIntel/1.0 (RSS Aggregator)")

	resp, err := client.Do(req)
	if err != nil {
		ch <- FetchResult{Source: source.Name, Err: err}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ch <- FetchResult{Source: source.Name, Err: fmt.Errorf("HTTP %d", resp.StatusCode)}
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5MB limit
	if err != nil {
		ch <- FetchResult{Source: source.Name, Err: err}
		return
	}

	articles, err := parseRSS(body, source)
	if err != nil {
		ch <- FetchResult{Source: source.Name, Err: err}
		return
	}

	ch <- FetchResult{Articles: articles, Source: source.Name}
}

// ─── RSS Parsing ─────────────────────────────────────────────────────────────

func parseRSS(data []byte, source FeedSource) ([]Article, error) {
	// Try RSS 2.0 first
	var rss rssFeed
	if err := xml.Unmarshal(data, &rss); err == nil && len(rss.Channel.Items) > 0 {
		return rssToArticles(rss.Channel.Items, source), nil
	}

	// Try Atom
	var atom atomFeed
	if err := xml.Unmarshal(data, &atom); err == nil && len(atom.Entries) > 0 {
		return atomToArticles(atom.Entries, source), nil
	}

	return nil, fmt.Errorf("unable to parse as RSS or Atom")
}

func rssToArticles(items []rssItem, source FeedSource) []Article {
	var articles []Article
	for _, item := range items {
		pubTime := parseTime(item.PubDate)
		desc := cleanSummary(item.Description)
		articles = append(articles, Article{
			Title:       strings.TrimSpace(item.Title),
			Link:        strings.TrimSpace(item.Link),
			Description: item.Description,
			Summary:     desc,
			Source:      source.Name,
			Category:    source.Category,
			PublishedAt: pubTime,
		})
	}
	return articles
}

func atomToArticles(entries []atomEntry, source FeedSource) []Article {
	var articles []Article
	for _, entry := range entries {
		pubTime := parseTime(entry.Updated)
		raw := entry.Summary
		if raw == "" {
			raw = entry.Content
		}
		desc := cleanSummary(raw)

		link := ""
		for _, l := range entry.Links {
			if l.Rel == "alternate" || l.Rel == "" {
				link = l.Href
				break
			}
		}
		if link == "" && len(entry.Links) > 0 {
			link = entry.Links[0].Href
		}

		articles = append(articles, Article{
			Title:       strings.TrimSpace(entry.Title),
			Link:        strings.TrimSpace(link),
			Description: raw,
			Summary:     desc,
			Source:      source.Name,
			Category:    source.Category,
			PublishedAt: pubTime,
		})
	}
	return articles
}

func parseTime(s string) time.Time {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
		time.RFC3339Nano,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
	}
	s = strings.TrimSpace(s)
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Now() // fallback for unparseable dates
}

// ─── Filtering & Dedup ──────────────────────────────────────────────────────

func filterByTime(articles []Article, cutoff time.Time) []Article {
	var filtered []Article
	for _, a := range articles {
		if a.PublishedAt.After(cutoff) {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

func deduplicateByURL(articles []Article) []Article {
	seen := make(map[string]bool)
	var unique []Article
	for _, a := range articles {
		normalized := normalizeURL(a.Link)
		if normalized != "" && !seen[normalized] {
			seen[normalized] = true
			unique = append(unique, a)
		}
	}
	return unique
}

func normalizeURL(u string) string {
	u = strings.TrimRight(u, "/")
	u = strings.SplitN(u, "?", 2)[0]
	u = strings.SplitN(u, "#", 2)[0]
	return strings.ToLower(u)
}

// ─── Scoring ─────────────────────────────────────────────────────────────────

func scoreArticle(a *Article) {
	text := strings.ToLower(a.Title + " " + a.Description)
	titleLower := strings.ToLower(a.Title)
	totalScore := 0
	var matched []string

	for keyword, points := range keywordScores {
		if strings.Contains(text, keyword) {
			totalScore += points
			matched = append(matched, keyword)
			// Title matches get bonus weight
			if strings.Contains(titleLower, keyword) {
				totalScore += points
			}
		}
	}

	// Recency bonus: articles from last 6 hours get +3
	if time.Since(a.PublishedAt) < 6*time.Hour {
		totalScore += 3
	}

	// Markets category bonus
	if a.Category == "Markets" {
		totalScore += 2
	}

	a.Score = totalScore
	a.Keywords = matched
}

// ─── Template Rendering ──────────────────────────────────────────────────────

func renderTemplate(nl Newsletter) (string, error) {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"formatTime": func(t time.Time) string {
			return t.In(sgt).Format("15:04 SGT, 02 Jan")
		},
	}

	// Try external file first
	tmplContent := embeddedTemplate
	exePath, _ := os.Executable()
	externalPath := filepath.Join(filepath.Dir(exePath), "template.html")
	if data, err := os.ReadFile(externalPath); err == nil {
		tmplContent = string(data)
	}

	tmpl, err := template.New("newsletter").Funcs(funcMap).Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("template parse: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nl); err != nil {
		return "", fmt.Errorf("template execute: %w", err)
	}

	return buf.String(), nil
}

// ─── Email ───────────────────────────────────────────────────────────────────

func sendEmail(cfg EmailConfig, htmlBody string, subject string) error {
	headers := map[string]string{
		"From":         cfg.FromAddr,
		"To":           strings.Join(cfg.Recipients, ", "),
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/html; charset=UTF-8",
	}

	var msg strings.Builder
	for k, v := range headers {
		fmt.Fprintf(&msg, "%s: %s\r\n", k, v)
	}
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)

	auth := smtp.PlainAuth("", cfg.Username, cfg.AppPassword, cfg.SMTPHost)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	return smtp.SendMail(addr, auth, cfg.FromAddr, cfg.Recipients, []byte(msg.String()))
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func cleanSummary(raw string) string {
	text := htmlTagRe.ReplaceAllString(raw, "")
	text = html.UnescapeString(text)
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 200 {
		text = text[:197] + "..."
	}
	return text
}

func loadEnvFile() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), "\"'")
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseRecipients(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func saveFallback(htmlBody string, now time.Time) {
	path := fmt.Sprintf("tech_intel_%s.html", now.Format("20060102_150405"))
	if err := os.WriteFile(path, []byte(htmlBody), 0644); err != nil {
		log.Printf("Failed to save fallback: %v", err)
	} else {
		log.Printf("Report saved to: %s", path)
	}
}
