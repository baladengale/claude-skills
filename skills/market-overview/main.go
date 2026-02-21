package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─── Yahoo Finance JSON Response Structs ─────────────────────────────────────

type YFChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				PreviousClose      float64 `json:"previousClose"`
				Currency           string  `json:"currency"`
				Symbol             string  `json:"symbol"`
			} `json:"meta"`
			Timestamps []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Close []*float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
			Events struct {
				Dividends map[string]struct {
					Amount float64 `json:"amount"`
					Date   int64   `json:"date"`
				} `json:"dividends"`
			} `json:"events"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

type YFQuoteResponse struct {
	QuoteResponse struct {
		Result []YFQuote `json:"result"`
		Error  *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"quoteResponse"`
}

type YFQuote struct {
	Symbol                 string  `json:"symbol"`
	LongName               string  `json:"longName"`
	ShortName              string  `json:"shortName"`
	RegularMarketPrice     float64 `json:"regularMarketPrice"`
	RegularMarketChange    float64 `json:"regularMarketChange"`
	RegularMarketChangePct float64 `json:"regularMarketChangePercent"`
	RegularMarketPrevClose float64 `json:"regularMarketPreviousClose"`
	RegularMarketOpen      float64 `json:"regularMarketOpen"`
	RegularMarketDayHigh   float64 `json:"regularMarketDayHigh"`
	RegularMarketDayLow    float64 `json:"regularMarketDayLow"`
	RegularMarketVolume    int64   `json:"regularMarketVolume"`
	AvgVolume3M            int64   `json:"averageDailyVolume3Month"`
	MarketCap              float64 `json:"marketCap"`
	FiftyTwoWeekHigh       float64 `json:"fiftyTwoWeekHigh"`
	FiftyTwoWeekLow        float64 `json:"fiftyTwoWeekLow"`
	TrailingPE             float64 `json:"trailingPE"`
	ForwardPE              float64 `json:"forwardPE"`
	EPSTrailing            float64 `json:"epsTrailingTwelveMonths"`
	EPSForward             float64 `json:"epsForward"`
	DividendRate           float64 `json:"trailingAnnualDividendRate"`
	DividendYield          float64 `json:"trailingAnnualDividendYield"`
	PriceToBook            float64 `json:"priceToBook"`
	Currency               string  `json:"currency"`
}

type YFSummaryResponse struct {
	QuoteSummary struct {
		Result []YFSummaryResult `json:"result"`
	} `json:"quoteSummary"`
}

type YFSummaryResult struct {
	AssetProfile *struct {
		Sector   string `json:"sector"`
		Industry string `json:"industry"`
	} `json:"assetProfile"`
	FinancialData *struct {
		RecommendationKey string `json:"recommendationKey"`
		TargetHighPrice   *YFRaw `json:"targetHighPrice"`
		TargetLowPrice    *YFRaw `json:"targetLowPrice"`
		TargetMeanPrice   *YFRaw `json:"targetMeanPrice"`
		TargetMedianPrice *YFRaw `json:"targetMedianPrice"`
		NumAnalysts       *YFRaw `json:"numberOfAnalystOpinions"`
		ProfitMargins     *YFRaw `json:"profitMargins"`
		RevenueGrowth     *YFRaw `json:"revenueGrowth"`
		EarningsGrowth    *YFRaw `json:"earningsGrowth"`
	} `json:"financialData"`
	DefaultKeyStatistics *struct {
		PegRatio          *YFRaw `json:"pegRatio"`
		Beta              *YFRaw `json:"beta"`
		EnterpriseValue   *YFRaw `json:"enterpriseValue"`
		SharesOutstanding *YFRaw `json:"sharesOutstanding"`
		PriceToSales      *YFRaw `json:"priceToSalesTrailing12Months"`
	} `json:"defaultKeyStatistics"`
	EarningsHistory *struct {
		History []struct {
			Quarter     *struct{ Fmt string } `json:"quarter"`
			EPSActual   *YFRaw                `json:"epsActual"`
			EPSEstimate *YFRaw                `json:"epsEstimate"`
			SurprisePct *YFRaw                `json:"surprisePercent"`
		} `json:"history"`
	} `json:"earningsHistory"`
	CalendarEvents *struct {
		Earnings *struct {
			EarningsDate []struct{ Raw int64 } `json:"earningsDate"`
		} `json:"earnings"`
	} `json:"calendarEvents"`
	IncomeStatementHistory *struct {
		Statements []YFFinStatement `json:"incomeStatementHistory"`
	} `json:"incomeStatementHistory"`
	IncomeStatementHistoryQuarterly *struct {
		Statements []YFFinStatement `json:"incomeStatementHistoryQuarterly"`
	} `json:"incomeStatementHistoryQuarterly"`
	CashflowStatementHistory *struct {
		Statements []YFCashflowStatement `json:"cashflowStatements"`
	} `json:"cashflowStatementHistory"`
	CashflowStatementHistoryQuarterly *struct {
		Statements []YFCashflowStatement `json:"cashflowStatements"`
	} `json:"cashflowStatementHistoryQuarterly"`
}

type YFRaw struct {
	Raw float64 `json:"raw"`
	Fmt string  `json:"fmt"`
}

type YFFinStatement struct {
	EndDate         *struct{ Fmt string } `json:"endDate"`
	TotalRevenue    *YFRaw                `json:"totalRevenue"`
	GrossProfit     *YFRaw                `json:"grossProfit"`
	OperatingIncome *YFRaw                `json:"operatingIncome"`
	NetIncome       *YFRaw                `json:"netIncome"`
	EBITDA          *YFRaw                `json:"ebitda"`
}

type YFCashflowStatement struct {
	EndDate             *struct{ Fmt string } `json:"endDate"`
	OperatingCashflow   *YFRaw                `json:"totalCashFromOperatingActivities"`
	InvestingCashflow   *YFRaw                `json:"totalCashflowsFromInvestingActivities"`
	FinancingCashflow   *YFRaw                `json:"totalCashFromFinancingActivities"`
	CapitalExpenditures *YFRaw                `json:"capitalExpenditures"`
	FreeCashFlow        *YFRaw                `json:"freeCashFlow"`
	EndCashPosition     *YFRaw                `json:"endCashPosition"`
}

// ─── Application Data Models ─────────────────────────────────────────────────

type TickerConfig struct {
	Symbol string
	Name   string
}

type SectionConfig struct {
	Name    string
	Tickers []TickerConfig
}

type MarketResult struct {
	Symbol         string
	Name           string
	Price          float64
	ChangePct      float64
	Historical     map[string]float64
	Week52High     float64
	Week52Low      float64
	PETrailing     float64
	PEForward      float64
	TargetHigh     float64
	TargetMean     float64
	TargetLow      float64
	Recommendation string
}

type EmailConfig struct {
	SMTPHost    string
	SMTPPort    int
	Username    string
	AppPassword string
	FromAddr    string
	Recipients  []string
}

// HTML template data models
type HTMLReport struct {
	Title     string
	Date      string
	Generated string
	ViewName  string
	Sections  []HTMLSection
	Movers    []HTMLMoversSection
	HasMovers bool
}

type HTMLSection struct {
	Name string
	Rows []HTMLRow
}

type HTMLRow struct {
	Name           string
	Price          string
	ChangePct      float64
	ChangePctStr   string
	ChangeArrow    string
	TrendPct       float64 // effective period: 1D if non-zero, else 1W/1M/3M
	Historical     []HTMLHistCell
	Week52         string
	PETrailing     string
	PEForward      string
	Target         string
	Recommendation string
	RecClass       string
}

type HTMLHistCell struct {
	Label string
	Value string
	Class string
}

type HTMLMoversSection struct {
	Region  string
	Gainers []HTMLMover
	Losers  []HTMLMover
}

type HTMLMover struct {
	Name      string
	ChangePct string
	Class     string
}

// ─── Constants & Configuration ───────────────────────────────────────────────

var sgt = time.FixedZone("SGT", 8*60*60)

var invertTickers = map[string]bool{"EURUSD=X": true, "GBPUSD=X": true}

var crossRates = map[string][2]string{
	"SGD/INR": {"USDINR=X", "USDSGD=X"},
	"SGD/MYR": {"USDMYR=X", "USDSGD=X"},
}

type HistPeriod struct {
	Days  int
	Label string
}

var historicalPeriods = []HistPeriod{
	{5, "1W"}, {21, "1M"}, {63, "3M"}, {126, "6M"},
	{252, "1Y"}, {504, "2Y"}, {1260, "5Y"},
}

var detailView = []SectionConfig{
	{Name: "US Markets", Tickers: []TickerConfig{
		{"^GSPC", "S&P 500"}, {"^DJI", "Dow Jones"}, {"^IXIC", "NASDAQ"},
		{"^RUT", "Russell 2000"}, {"^VIX", "VIX"},
	}},
	{Name: "European Markets", Tickers: []TickerConfig{
		{"^FTSE", "FTSE 100"}, {"^GDAXI", "DAX"}, {"^FCHI", "CAC 40"},
		{"^STOXX50E", "Euro Stoxx 50"},
	}},
	{Name: "Asian Markets", Tickers: []TickerConfig{
		{"^N225", "Nikkei 225"}, {"^HSI", "Hang Seng"}, {"000001.SS", "Shanghai"},
		{"^STI", "STI Singapore"}, {"^BSESN", "Sensex"}, {"^NSEI", "Nifty 50"},
		{"^KS11", "KOSPI"}, {"^TWII", "TAIEX"},
	}},
	{Name: "Commodities & Crypto", Tickers: []TickerConfig{
		{"GC=F", "Gold"}, {"SI=F", "Silver"}, {"CL=F", "Crude Oil WTI"},
		{"BZ=F", "Brent Crude"}, {"BTC-USD", "Bitcoin"}, {"ETH-USD", "Ethereum"},
	}},
	{Name: "Currencies", Tickers: []TickerConfig{
		{"EURUSD=X", "USD/EUR"}, {"GBPUSD=X", "USD/GBP"}, {"USDJPY=X", "USD/JPY"},
		{"USDCNY=X", "USD/CNY"}, {"USDINR=X", "USD/INR"}, {"USDSGD=X", "USD/SGD"},
		{"USDMYR=X", "USD/MYR"}, {"SGD/INR", "SGD/INR"}, {"SGD/MYR", "SGD/MYR"},
	}},
}

var summaryView = []SectionConfig{
	{Name: "Key Indices", Tickers: []TickerConfig{
		{"^GSPC", "S&P 500"}, {"^IXIC", "NASDAQ"}, {"^HSI", "Hang Seng"},
		{"^STI", "STI Singapore"}, {"^BSESN", "Sensex"}, {"^NSEI", "Nifty 50"},
	}},
	{Name: "Commodities & Crypto", Tickers: []TickerConfig{
		{"GC=F", "Gold"}, {"SI=F", "Silver"}, {"CL=F", "Crude Oil WTI"},
		{"BZ=F", "Brent Crude"}, {"BTC-USD", "Bitcoin"}, {"ETH-USD", "Ethereum"},
	}},
	{Name: "Currencies", Tickers: []TickerConfig{
		{"USDINR=X", "USD/INR"}, {"USDSGD=X", "USD/SGD"},
		{"SGD/INR", "SGD/INR"}, {"SGD/MYR", "SGD/MYR"},
	}},
}

var portfolioView = []SectionConfig{
	{Name: "My Portfolio", Tickers: []TickerConfig{
		{"TSLA", "Tesla"}, {"NVDA", "NVIDIA"}, {"V", "Visa"}, {"MSFT", "Microsoft"},
		{"META", "Meta"}, {"GOOGL", "Google"}, {"AMZN", "Amazon"}, {"AMD", "AMD"},
		{"AVGO", "Broadcom"}, {"AAPL", "Apple"},
	}},
}

var topMoversStocks = []SectionConfig{
	{Name: "US Stocks", Tickers: []TickerConfig{
		{"TSLA", "Tesla"}, {"NVDA", "NVIDIA"}, {"AAPL", "Apple"}, {"MSFT", "Microsoft"},
		{"GOOGL", "Google"}, {"AMZN", "Amazon"}, {"META", "Meta"}, {"AMD", "AMD"},
		{"AVGO", "Broadcom"}, {"V", "Visa"}, {"JPM", "JPMorgan"}, {"BAC", "BofA"},
		{"WMT", "Walmart"}, {"DIS", "Disney"}, {"NFLX", "Netflix"},
	}},
	{Name: "Singapore Stocks", Tickers: []TickerConfig{
		{"D05.SI", "DBS"}, {"O39.SI", "OCBC"}, {"U11.SI", "UOB"},
		{"Z74.SI", "Singtel"}, {"C6L.SI", "SIA"}, {"C38U.SI", "CapitaLand"},
		{"G13.SI", "Genting SG"}, {"S58.SI", "SATS"},
	}},
	{Name: "India Stocks", Tickers: []TickerConfig{
		{"RELIANCE.NS", "Reliance"}, {"TCS.NS", "TCS"}, {"INFY.NS", "Infosys"},
		{"HDFCBANK.NS", "HDFC Bank"}, {"ICICIBANK.NS", "ICICI Bank"},
		{"HINDUNILVR.NS", "HUL"}, {"ITC.NS", "ITC"}, {"SBIN.NS", "SBI"},
		{"BHARTIARTL.NS", "Bharti Airtel"}, {"WIPRO.NS", "Wipro"},
	}},
}

const (
	maxRetries  = 3
	retryDelay  = 2 * time.Second
	httpTimeout = 15 * time.Second
	userAgent   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
	maxSem      = 10
)

//go:embed template.html
var embeddedTemplate string

// Yahoo Finance auth (cookie + crumb)
var (
	yahooCookie string
	yahooCrumb  string
)

func initYahooAuth() {
	client := &http.Client{Timeout: httpTimeout}
	req, _ := http.NewRequest("GET", "https://fc.yahoo.com", nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("WARN: Yahoo cookie fetch failed: %v", err)
		return
	}
	resp.Body.Close()
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "A3" {
			yahooCookie = cookie.Name + "=" + cookie.Value
			break
		}
	}
	if yahooCookie == "" {
		log.Println("WARN: No A3 cookie received from Yahoo")
		return
	}
	req2, _ := http.NewRequest("GET", "https://query2.finance.yahoo.com/v1/test/getcrumb", nil)
	req2.Header.Set("User-Agent", userAgent)
	req2.Header.Set("Cookie", yahooCookie)
	resp2, err := client.Do(req2)
	if err != nil {
		log.Printf("WARN: Yahoo crumb fetch failed: %v", err)
		return
	}
	defer resp2.Body.Close()
	body, _ := io.ReadAll(resp2.Body)
	yahooCrumb = strings.TrimSpace(string(body))
	if yahooCrumb != "" {
		log.Printf("Yahoo auth initialized (crumb: %s...)", yahooCrumb[:4])
	}
}

// ─── Main ────────────────────────────────────────────────────────────────────

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	summary := flag.Bool("s", false, "Summary view (key indices only)")
	portfolio := flag.Bool("p", false, "Portfolio view (your stocks)")
	noMovers := flag.Bool("no-movers", false, "Skip stock movers section")
	stock := flag.String("t", "", "Detailed stock info for SYMBOL")
	dividends := flag.String("d", "", "Dividend history for SYMBOL")
	earnings := flag.String("e", "", "Earnings dates/EPS for SYMBOL")
	financials := flag.String("f", "", "Income statement for SYMBOL")
	cashflow := flag.String("c", "", "Cashflow statement for SYMBOL")
	quarterly := flag.Bool("q", false, "Quarterly data (for -f and -c)")
	noEmail := flag.Bool("no-email", false, "Skip email delivery")
	emailOnly := flag.Bool("email-only", false, "Email only, skip terminal")
	flag.Parse()

	loadEnvFile()
	initYahooAuth()

	cfg := EmailConfig{
		SMTPHost:    "smtp.gmail.com",
		SMTPPort:    587,
		Username:    envOrDefault("GMAIL_USER", "dengalebr@gmail.com"),
		AppPassword: os.Getenv("GMAIL_APP_PASSWORD"),
		FromAddr:    envOrDefault("GMAIL_USER", "dengalebr@gmail.com"),
		Recipients:  parseRecipients(envOrDefault("MARKET_RECIPIENTS", "dengalebr@gmail.com,badengal@visa.com")),
	}

	now := time.Now().In(sgt)

	switch {
	case *stock != "":
		runStockDetail(strings.ToUpper(*stock), cfg, now, *noEmail, *emailOnly)
	case *dividends != "":
		runDividends(strings.ToUpper(*dividends), cfg, now, *noEmail, *emailOnly)
	case *earnings != "":
		runEarnings(strings.ToUpper(*earnings), cfg, now, *noEmail, *emailOnly)
	case *financials != "":
		runFinancials(strings.ToUpper(*financials), *quarterly, cfg, now, *noEmail, *emailOnly)
	case *cashflow != "":
		runCashflow(strings.ToUpper(*cashflow), *quarterly, cfg, now, *noEmail, *emailOnly)
	default:
		var view []SectionConfig
		var viewName string
		switch {
		case *portfolio:
			view, viewName = portfolioView, "My Portfolio"
		case *summary:
			view, viewName = summaryView, "Market Summary"
		default:
			view, viewName = detailView, "World Markets Overview"
		}
		showMovers := !*noMovers
		runMarketOverview(view, viewName, showMovers, cfg, now, *noEmail, *emailOnly)
	}
}

// ─── Market Overview Mode ────────────────────────────────────────────────────

func runMarketOverview(view []SectionConfig, viewName string, showMovers bool, cfg EmailConfig, now time.Time, noEmail, emailOnly bool) {
	isPortfolioView := viewName == "My Portfolio"

	// For the HTML email, combine the requested view with portfolio (unless already portfolio view)
	emailView := view
	if !isPortfolioView {
		emailView = append(append([]SectionConfig{}, view...), portfolioView...)
	}

	// Collect symbols for the combined email view + movers
	allSymbols := collectSymbols(emailView, showMovers)
	log.Printf("Fetching %d symbols for %s...", len(allSymbols), viewName)

	charts := fetchChartsConcurrent(allSymbols)
	log.Printf("Got chart data for %d symbols", len(charts))

	// Build results for the combined view (includes portfolio symbols)
	results := buildMarketResults(charts, emailView, showMovers)

	// Terminal output uses original view only
	if !emailOnly {
		printMarketOverview(results, view, viewName, showMovers, now)
	}

	// HTML email uses combined view (overview + portfolio + movers)
	if !noEmail && cfg.AppPassword != "" {
		htmlBody := renderMarketHTML(results, emailView, viewName, showMovers, now)
		subject := fmt.Sprintf("Market Overview: %s - %s", viewName, now.Format("02 Jan 2006"))
		if err := sendEmail(cfg, htmlBody, subject); err != nil {
			log.Printf("Email failed: %v", err)
			saveFallback(htmlBody, now, "market_overview")
		} else {
			log.Printf("Email sent to %v", cfg.Recipients)
		}
	}
}

// ─── Yahoo Finance API Layer ─────────────────────────────────────────────────

func yahooGet(rawURL string) ([]byte, error) {
	client := &http.Client{Timeout: httpTimeout}
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		req, _ := http.NewRequest("GET", rawURL, nil)
		req.Header.Set("User-Agent", userAgent)
		if yahooCookie != "" {
			req.Header.Set("Cookie", yahooCookie)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(retryDelay * time.Duration(attempt+1))
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
		resp.Body.Close()
		if resp.StatusCode == 429 {
			lastErr = fmt.Errorf("rate limited (429)")
			time.Sleep(retryDelay * time.Duration(attempt+1))
			continue
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return body, nil
	}
	return nil, fmt.Errorf("max retries: %v", lastErr)
}

func fetchQuotes(symbols []string) (map[string]YFQuote, error) {
	encoded := make([]string, len(symbols))
	for i, s := range symbols {
		encoded[i] = url.QueryEscape(s)
	}
	u := "https://query2.finance.yahoo.com/v7/finance/quote?symbols=" + strings.Join(encoded, ",")
	if yahooCrumb != "" {
		u += "&crumb=" + url.QueryEscape(yahooCrumb)
	}
	data, err := yahooGet(u)
	if err != nil {
		return nil, err
	}
	var resp YFQuoteResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	result := make(map[string]YFQuote)
	for _, q := range resp.QuoteResponse.Result {
		result[q.Symbol] = q
	}
	return result, nil
}

type ChartData struct {
	Closes        []float64
	CurrentPrice  float64
	PreviousClose float64
	Currency      string
}

func fetchChart(symbol string) (*ChartData, error) {
	u := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?range=10y&interval=1d",
		url.QueryEscape(symbol))
	data, err := yahooGet(u)
	if err != nil {
		return nil, err
	}
	var resp YFChartResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if len(resp.Chart.Result) == 0 || len(resp.Chart.Result[0].Indicators.Quote) == 0 {
		return nil, fmt.Errorf("no chart data")
	}
	result := resp.Chart.Result[0]
	rawCloses := result.Indicators.Quote[0].Close
	closes := make([]float64, 0, len(rawCloses))
	for _, c := range rawCloses {
		if c != nil {
			closes = append(closes, *c)
		}
	}
	if len(closes) == 0 {
		return nil, fmt.Errorf("empty closes")
	}
	return &ChartData{
		Closes:        closes,
		CurrentPrice:  result.Meta.RegularMarketPrice,
		PreviousClose: result.Meta.PreviousClose,
		Currency:      result.Meta.Currency,
	}, nil
}

func fetchChartsConcurrent(symbols []string) map[string]*ChartData {
	results := make(map[string]*ChartData)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxSem)

	for _, sym := range symbols {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			cd, err := fetchChart(s)
			if err != nil {
				log.Printf("WARN: chart %s: %v", s, err)
				return
			}
			mu.Lock()
			results[s] = cd
			mu.Unlock()
		}(sym)
	}
	wg.Wait()
	return results
}

func fetchQuoteSummary(symbol string, modules []string) (*YFSummaryResult, error) {
	u := fmt.Sprintf("https://query2.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=%s",
		url.QueryEscape(symbol), strings.Join(modules, ","))
	if yahooCrumb != "" {
		u += "&crumb=" + url.QueryEscape(yahooCrumb)
	}
	data, err := yahooGet(u)
	if err != nil {
		return nil, err
	}
	var resp YFSummaryResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if len(resp.QuoteSummary.Result) == 0 {
		return nil, fmt.Errorf("no summary data")
	}
	return &resp.QuoteSummary.Result[0], nil
}

// ─── Data Processing ─────────────────────────────────────────────────────────

func collectSymbols(view []SectionConfig, includeMovers bool) []string {
	seen := make(map[string]bool)
	var symbols []string
	add := func(s string) {
		if _, isCross := crossRates[s]; isCross {
			return
		}
		if !seen[s] {
			seen[s] = true
			symbols = append(symbols, s)
		}
	}
	for _, sec := range view {
		for _, t := range sec.Tickers {
			add(t.Symbol)
		}
	}
	// Add cross-rate dependencies
	for _, pair := range crossRates {
		add(pair[0])
		add(pair[1])
	}
	if includeMovers {
		for _, sec := range topMoversStocks {
			for _, t := range sec.Tickers {
				add(t.Symbol)
			}
		}
	}
	return symbols
}

func buildMarketResults(charts map[string]*ChartData, view []SectionConfig, includeMovers bool) map[string]*MarketResult {
	results := make(map[string]*MarketResult)

	process := func(symbol, name string) {
		if _, exists := results[symbol]; exists {
			return
		}
		// Check if cross-rate
		if pair, isCross := crossRates[symbol]; isCross {
			cr := computeCrossRate(pair[0], pair[1], charts)
			if cr != nil {
				cr.Name = name
				cr.Symbol = symbol
				results[symbol] = cr
			}
			return
		}
		cd, ok := charts[symbol]
		if !ok || cd.CurrentPrice == 0 {
			return
		}
		price := cd.CurrentPrice
		prevClose := cd.PreviousClose
		invert := invertTickers[symbol]
		if invert && price != 0 {
			price = 1.0 / price
			if prevClose != 0 {
				prevClose = 1.0 / prevClose
			}
		}
		var changePct float64
		if prevClose != 0 {
			changePct = ((price - prevClose) / prevClose) * 100
		}
		mr := &MarketResult{
			Symbol:     symbol,
			Name:       name,
			Price:      price,
			ChangePct:  changePct,
			Historical: make(map[string]float64),
		}
		// Historical from chart closes
		closes := cd.Closes
		if len(closes) > 1 {
			current := closes[len(closes)-1]
			if invert && current != 0 {
				current = 1.0 / current
			}
			for _, p := range historicalPeriods {
				idx := len(closes) - 1 - p.Days
				if idx >= 0 {
					hist := closes[idx]
					if invert && hist != 0 {
						hist = 1.0 / hist
					}
					if hist != 0 {
						mr.Historical[p.Label] = ((current - hist) / hist) * 100
					}
				}
			}
		}
		results[symbol] = mr
	}

	for _, sec := range view {
		for _, t := range sec.Tickers {
			process(t.Symbol, t.Name)
		}
	}
	if includeMovers {
		for _, sec := range topMoversStocks {
			for _, t := range sec.Tickers {
				process(t.Symbol, t.Name)
			}
		}
	}
	return results
}

func computeCrossRate(baseSym, quoteSym string, charts map[string]*ChartData) *MarketResult {
	bc, bok := charts[baseSym]
	qc, qok := charts[quoteSym]
	if !bok || !qok || qc.CurrentPrice == 0 {
		return nil
	}
	price := bc.CurrentPrice / qc.CurrentPrice
	var changePct float64
	if bc.PreviousClose != 0 && qc.PreviousClose != 0 {
		prevCross := bc.PreviousClose / qc.PreviousClose
		if prevCross != 0 {
			changePct = ((price - prevCross) / prevCross) * 100
		}
	}
	mr := &MarketResult{
		Price:      price,
		ChangePct:  changePct,
		Historical: make(map[string]float64),
	}
	if len(bc.Closes) > 1 && len(qc.Closes) > 1 {
		current := bc.Closes[len(bc.Closes)-1] / qc.Closes[len(qc.Closes)-1]
		for _, p := range historicalPeriods {
			bidx := len(bc.Closes) - 1 - p.Days
			qidx := len(qc.Closes) - 1 - p.Days
			if bidx >= 0 && qidx >= 0 && qc.Closes[qidx] != 0 {
				hist := bc.Closes[bidx] / qc.Closes[qidx]
				if hist != 0 {
					mr.Historical[p.Label] = ((current - hist) / hist) * 100
				}
			}
		}
	}
	return mr
}

// ─── Terminal Rendering ──────────────────────────────────────────────────────

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiDim    = "\033[2m"
	ansiBgBlue = "\033[44m"
	ansiWhite  = "\033[37m"
)

func colorPct(val float64) string {
	if val > 0 {
		return ansiGreen
	} else if val < 0 {
		return ansiRed
	}
	return ansiDim
}

func changeArrow(val float64) string {
	switch {
	case val > 1.5:
		return "▲▲"
	case val > 0:
		return "▲"
	case val == 0:
		return "━"
	case val > -1.5:
		return "▼"
	default:
		return "▼▼"
	}
}

func fmtPct(val float64, valid bool) string {
	if !valid {
		return fmt.Sprintf("%s  -%s", ansiDim, ansiReset)
	}
	sign := ""
	if val > 0 {
		sign = "+"
	}
	return fmt.Sprintf("%s%s%.1f%%%s", colorPct(val), sign, val, ansiReset)
}

func fmtPrice(val float64) string {
	if val == 0 {
		return "-"
	}
	if val >= 10000 {
		return addCommas(fmt.Sprintf("%.0f", val))
	} else if val >= 100 {
		return addCommas(fmt.Sprintf("%.2f", val))
	}
	return fmt.Sprintf("%.4f", val)
}

func addCommas(s string) string {
	parts := strings.Split(s, ".")
	integer := parts[0]
	var result []byte
	for i, c := range integer {
		if i > 0 && (len(integer)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	if len(parts) > 1 {
		return string(result) + "." + parts[1]
	}
	return string(result)
}

func fmtLargeNum(val float64) string {
	abs := math.Abs(val)
	switch {
	case abs >= 1e12:
		return fmt.Sprintf("%.2fT", val/1e12)
	case abs >= 1e9:
		return fmt.Sprintf("%.2fB", val/1e9)
	case abs >= 1e6:
		return fmt.Sprintf("%.2fM", val/1e6)
	default:
		return fmt.Sprintf("%.0f", val)
	}
}

func printMarketOverview(results map[string]*MarketResult, view []SectionConfig, viewName string, showMovers bool, now time.Time) {
	fmt.Printf("\n%s%s%s %s %s%s\n", ansiBold, ansiBgBlue, ansiWhite,
		viewName, now.Format("Monday, 02 Jan 2006 15:04 SGT"), ansiReset)
	fmt.Println()

	periods := []string{"1W", "1M", "3M", "6M", "1Y", "2Y", "5Y"}

	for _, sec := range view {
		fmt.Printf("%s%s=== %s ===%s\n", ansiBold, ansiYellow, sec.Name, ansiReset)
		fmt.Printf("%-18s %12s %8s", "Market", "Price", "1D")
		for _, p := range periods {
			fmt.Printf(" %7s", p)
		}
		fmt.Println()
		fmt.Println(strings.Repeat("─", 90))

		for _, t := range sec.Tickers {
			mr, ok := results[t.Symbol]
			if !ok {
				continue
			}
			fmt.Printf("%-18s %12s %s%-7s %s%s",
				mr.Name, fmtPrice(mr.Price),
				colorPct(mr.ChangePct), fmt.Sprintf("%+.1f%%", mr.ChangePct),
				changeArrow(mr.ChangePct), ansiReset)
			for _, p := range periods {
				val, exists := mr.Historical[p]
				fmt.Printf(" %s", fmtPct(val, exists))
			}
			fmt.Println()
		}
		fmt.Println()
	}

	if showMovers {
		printMovers(results)
	}
}

func printMovers(results map[string]*MarketResult) {
	fmt.Printf("%s%s=== Top Movers ===%s\n\n", ansiBold, ansiYellow, ansiReset)

	for _, sec := range topMoversStocks {
		type mover struct {
			name      string
			changePct float64
		}
		var movers []mover
		for _, t := range sec.Tickers {
			if mr, ok := results[t.Symbol]; ok {
				movers = append(movers, mover{mr.Name, effectiveMoverPct(mr)})
			}
		}
		sort.Slice(movers, func(i, j int) bool { return movers[i].changePct > movers[j].changePct })

		fmt.Printf("  %s%s%s\n", ansiBold, sec.Name, ansiReset)
		top := 5
		if len(movers) < 5 {
			top = len(movers)
		}
		fmt.Printf("  %sGainers:%s ", ansiGreen, ansiReset)
		for i := 0; i < top; i++ {
			fmt.Printf("%s(%s%+.1f%%%s) ", movers[i].name, colorPct(movers[i].changePct), movers[i].changePct, ansiReset)
		}
		fmt.Println()
		fmt.Printf("  %sLosers: %s ", ansiRed, ansiReset)
		for i := len(movers) - 1; i >= len(movers)-top && i >= 0; i-- {
			fmt.Printf("%s(%s%+.1f%%%s) ", movers[i].name, colorPct(movers[i].changePct), movers[i].changePct, ansiReset)
		}
		fmt.Printf("\n\n")
	}
}

// ─── Single-Stock Views ──────────────────────────────────────────────────────

func runStockDetail(symbol string, cfg EmailConfig, now time.Time, noEmail, emailOnly bool) {
	log.Printf("Fetching stock detail for %s...", symbol)
	cd, err := fetchChart(symbol)
	if err != nil {
		log.Fatalf("Failed to fetch chart: %v", err)
	}

	summary, _ := fetchQuoteSummary(symbol, []string{
		"assetProfile", "financialData", "defaultKeyStatistics",
	})

	price := cd.CurrentPrice
	prevClose := cd.PreviousClose
	change := price - prevClose
	var changePct float64
	if prevClose != 0 {
		changePct = (change / prevClose) * 100
	}

	if !emailOnly {
		fmt.Printf("\n%s%s (%s)%s\n\n", ansiBold, symbol, cd.Currency, ansiReset)

		fmt.Printf("  Price:        %s (%s%+.2f / %+.2f%%%s)\n",
			fmtPrice(price), colorPct(changePct), change, changePct, ansiReset)
		fmt.Printf("  Prev Close:   %s\n", fmtPrice(prevClose))

		if summary != nil && summary.AssetProfile != nil {
			fmt.Printf("  Sector:       %s\n", summary.AssetProfile.Sector)
			fmt.Printf("  Industry:     %s\n", summary.AssetProfile.Industry)
		}

		if summary != nil {
			if kd := summary.DefaultKeyStatistics; kd != nil {
				fmt.Println()
				fmt.Printf("  %sKey Statistics%s\n", ansiBold, ansiReset)
				if kd.PegRatio != nil {
					fmt.Printf("  PEG Ratio:    %.2f\n", kd.PegRatio.Raw)
				}
				if kd.Beta != nil {
					fmt.Printf("  Beta:         %.2f\n", kd.Beta.Raw)
				}
				if kd.EnterpriseValue != nil {
					fmt.Printf("  EV:           %s\n", fmtLargeNum(kd.EnterpriseValue.Raw))
				}
			}
			if fd := summary.FinancialData; fd != nil {
				fmt.Println()
				fmt.Printf("  %sAnalyst Targets%s\n", ansiBold, ansiReset)
				if fd.TargetLowPrice != nil && fd.TargetMeanPrice != nil && fd.TargetHighPrice != nil {
					fmt.Printf("  Low/Mean/High: %s%.2f%s / %s%.2f%s / %s%.2f%s\n",
						ansiRed, fd.TargetLowPrice.Raw, ansiReset,
						ansiYellow, fd.TargetMeanPrice.Raw, ansiReset,
						ansiGreen, fd.TargetHighPrice.Raw, ansiReset)
					upside := ((fd.TargetMeanPrice.Raw - price) / price) * 100
					fmt.Printf("  Upside:        %s%+.1f%%%s\n", colorPct(upside), upside, ansiReset)
				}
				if fd.RecommendationKey != "" {
					rec := strings.ToUpper(fd.RecommendationKey)
					fmt.Printf("  Recommendation: %s%s%s\n", recColor(rec), rec, ansiReset)
				}
				if fd.ProfitMargins != nil {
					fmt.Printf("  Profit Margin: %.1f%%\n", fd.ProfitMargins.Raw*100)
				}
				if fd.RevenueGrowth != nil {
					fmt.Printf("  Revenue Growth: %s%+.1f%%%s\n", colorPct(fd.RevenueGrowth.Raw), fd.RevenueGrowth.Raw*100, ansiReset)
				}
				if fd.EarningsGrowth != nil {
					fmt.Printf("  Earnings Growth: %s%+.1f%%%s\n", colorPct(fd.EarningsGrowth.Raw), fd.EarningsGrowth.Raw*100, ansiReset)
				}
			}
		}

		// Historical performance
		if len(cd.Closes) > 1 {
			fmt.Println()
			fmt.Printf("  %sHistorical Performance%s\n", ansiBold, ansiReset)
			current := cd.Closes[len(cd.Closes)-1]
			for _, p := range historicalPeriods {
				idx := len(cd.Closes) - 1 - p.Days
				if idx >= 0 && cd.Closes[idx] != 0 {
					pct := ((current - cd.Closes[idx]) / cd.Closes[idx]) * 100
					fmt.Printf("  %-6s %s\n", p.Label+":", fmtPct(pct, true))
				}
			}
		}
		fmt.Println()
	}

	if !noEmail && cfg.AppPassword != "" {
		// Build a minimal YFQuote for the HTML renderer
		q := YFQuote{
			Symbol:                 symbol,
			RegularMarketPrice:     price,
			RegularMarketChange:    change,
			RegularMarketChangePct: changePct,
			RegularMarketPrevClose: prevClose,
			Currency:               cd.Currency,
		}
		htmlBody := renderStockDetailHTML(q, summary, now)
		subject := fmt.Sprintf("Stock Detail: %s - %s", symbol, now.Format("02 Jan 2006"))
		if err := sendEmail(cfg, htmlBody, subject); err != nil {
			log.Printf("Email failed: %v", err)
		} else {
			log.Printf("Email sent to %v", cfg.Recipients)
		}
	}
}

// getSymbolName fetches chart meta for a symbol and returns the price as a fallback name
func getChartPrice(symbol string) (float64, string) {
	cd, err := fetchChart(symbol)
	if err != nil {
		return 0, ""
	}
	return cd.CurrentPrice, cd.Currency
}

func runDividends(symbol string, cfg EmailConfig, now time.Time, noEmail, emailOnly bool) {
	log.Printf("Fetching dividends for %s...", symbol)
	u := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?range=max&interval=1mo&events=div",
		url.QueryEscape(symbol))
	data, err := yahooGet(u)
	if err != nil {
		log.Fatalf("Failed: %v", err)
	}
	var resp YFChartResponse
	json.Unmarshal(data, &resp)

	if !emailOnly {
		fmt.Printf("\n%s%s Dividend History%s\n\n", ansiBold, symbol, ansiReset)
		if len(resp.Chart.Result) > 0 {
			divs := resp.Chart.Result[0].Events.Dividends
			type divEntry struct {
				date   time.Time
				amount float64
			}
			var entries []divEntry
			for _, d := range divs {
				entries = append(entries, divEntry{time.Unix(d.Date, 0), d.Amount})
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].date.After(entries[j].date) })
			fmt.Printf("  %-15s %10s\n", "Date", "Amount")
			fmt.Println("  " + strings.Repeat("─", 28))
			limit := 12
			if len(entries) < limit {
				limit = len(entries)
			}
			for i := 0; i < limit; i++ {
				fmt.Printf("  %-15s $%9.4f\n", entries[i].date.Format("02 Jan 2006"), entries[i].amount)
			}
			fmt.Printf("\n  Total dividends on record: %d\n", len(entries))
		}
		fmt.Println()
	}
}

func runEarnings(symbol string, cfg EmailConfig, now time.Time, noEmail, emailOnly bool) {
	log.Printf("Fetching earnings for %s...", symbol)
	summary, err := fetchQuoteSummary(symbol, []string{"earningsHistory", "calendarEvents"})
	if err != nil {
		log.Fatalf("Failed: %v", err)
	}

	if !emailOnly {
		fmt.Printf("\n%s%s Earnings%s\n\n", ansiBold, symbol, ansiReset)

		if summary.CalendarEvents != nil && summary.CalendarEvents.Earnings != nil {
			dates := summary.CalendarEvents.Earnings.EarningsDate
			if len(dates) > 0 {
				fmt.Printf("  Next Earnings: %s\n\n", time.Unix(dates[0].Raw, 0).Format("02 Jan 2006"))
			}
		}

		if summary.EarningsHistory != nil {
			fmt.Printf("  %-12s %10s %10s %10s\n", "Quarter", "Estimate", "Actual", "Surprise")
			fmt.Println("  " + strings.Repeat("─", 46))
			for _, h := range summary.EarningsHistory.History {
				qtr := "-"
				if h.Quarter != nil {
					qtr = h.Quarter.Fmt
				}
				est, act, sur := "-", "-", "-"
				if h.EPSEstimate != nil {
					est = fmt.Sprintf("%.2f", h.EPSEstimate.Raw)
				}
				if h.EPSActual != nil {
					act = fmt.Sprintf("%.2f", h.EPSActual.Raw)
				}
				if h.SurprisePct != nil {
					sur = fmt.Sprintf("%+.1f%%", h.SurprisePct.Raw*100)
				}
				fmt.Printf("  %-12s %10s %10s %10s\n", qtr, est, act, sur)
			}
		}
		fmt.Println()
	}
}

func runFinancials(symbol string, quarterly bool, cfg EmailConfig, now time.Time, noEmail, emailOnly bool) {
	log.Printf("Fetching financials for %s (quarterly=%v)...", symbol, quarterly)
	mod := "incomeStatementHistory"
	if quarterly {
		mod = "incomeStatementHistoryQuarterly"
	}
	summary, err := fetchQuoteSummary(symbol, []string{mod})
	if err != nil {
		log.Fatalf("Failed: %v", err)
	}

	var stmts []YFFinStatement
	if quarterly && summary.IncomeStatementHistoryQuarterly != nil {
		stmts = summary.IncomeStatementHistoryQuarterly.Statements
	} else if summary.IncomeStatementHistory != nil {
		stmts = summary.IncomeStatementHistory.Statements
	}

	if !emailOnly {
		label := "Yearly"
		if quarterly {
			label = "Quarterly"
		}
		fmt.Printf("\n%s%s Income Statement - %s%s\n\n", ansiBold, symbol, label, ansiReset)

		if len(stmts) > 0 {
			// Print header
			fmt.Printf("  %-20s", "")
			for _, s := range stmts {
				date := "-"
				if s.EndDate != nil {
					date = s.EndDate.Fmt
				}
				fmt.Printf(" %14s", date)
			}
			fmt.Println()
			fmt.Println("  " + strings.Repeat("─", 20+15*len(stmts)))

			printFinRow := func(label string, getter func(YFFinStatement) *YFRaw) {
				fmt.Printf("  %-20s", label)
				for _, s := range stmts {
					v := getter(s)
					if v != nil {
						fmt.Printf(" %14s", fmtLargeNum(v.Raw))
					} else {
						fmt.Printf(" %14s", "-")
					}
				}
				fmt.Println()
			}
			printFinRow("Total Revenue", func(s YFFinStatement) *YFRaw { return s.TotalRevenue })
			printFinRow("Gross Profit", func(s YFFinStatement) *YFRaw { return s.GrossProfit })
			printFinRow("Operating Income", func(s YFFinStatement) *YFRaw { return s.OperatingIncome })
			printFinRow("Net Income", func(s YFFinStatement) *YFRaw { return s.NetIncome })
			printFinRow("EBITDA", func(s YFFinStatement) *YFRaw { return s.EBITDA })
		}
		fmt.Println()
	}
}

func runCashflow(symbol string, quarterly bool, cfg EmailConfig, now time.Time, noEmail, emailOnly bool) {
	log.Printf("Fetching cashflow for %s (quarterly=%v)...", symbol, quarterly)
	mod := "cashflowStatementHistory"
	if quarterly {
		mod = "cashflowStatementHistoryQuarterly"
	}
	summary, err := fetchQuoteSummary(symbol, []string{mod})
	if err != nil {
		log.Fatalf("Failed: %v", err)
	}

	var stmts []YFCashflowStatement
	if quarterly && summary.CashflowStatementHistoryQuarterly != nil {
		stmts = summary.CashflowStatementHistoryQuarterly.Statements
	} else if summary.CashflowStatementHistory != nil {
		stmts = summary.CashflowStatementHistory.Statements
	}

	if !emailOnly {
		label := "Yearly"
		if quarterly {
			label = "Quarterly"
		}
		fmt.Printf("\n%s%s Cashflow Statement - %s%s\n\n", ansiBold, symbol, label, ansiReset)

		if len(stmts) > 0 {
			fmt.Printf("  %-22s", "")
			for _, s := range stmts {
				date := "-"
				if s.EndDate != nil {
					date = s.EndDate.Fmt
				}
				fmt.Printf(" %14s", date)
			}
			fmt.Println()
			fmt.Println("  " + strings.Repeat("─", 22+15*len(stmts)))

			printCfRow := func(label string, getter func(YFCashflowStatement) *YFRaw) {
				fmt.Printf("  %-22s", label)
				for _, s := range stmts {
					v := getter(s)
					if v != nil {
						fmt.Printf(" %14s", fmtLargeNum(v.Raw))
					} else {
						fmt.Printf(" %14s", "-")
					}
				}
				fmt.Println()
			}
			printCfRow("Operating CF", func(s YFCashflowStatement) *YFRaw { return s.OperatingCashflow })
			printCfRow("Investing CF", func(s YFCashflowStatement) *YFRaw { return s.InvestingCashflow })
			printCfRow("Financing CF", func(s YFCashflowStatement) *YFRaw { return s.FinancingCashflow })
			printCfRow("Capital Expenditures", func(s YFCashflowStatement) *YFRaw { return s.CapitalExpenditures })
			printCfRow("Free Cash Flow", func(s YFCashflowStatement) *YFRaw { return s.FreeCashFlow })
			printCfRow("End Cash Position", func(s YFCashflowStatement) *YFRaw { return s.EndCashPosition })
		}
		fmt.Println()
	}
}

func recColor(rec string) string {
	switch {
	case strings.Contains(rec, "BUY"):
		return ansiGreen
	case strings.Contains(rec, "HOLD"):
		return ansiYellow
	case strings.Contains(rec, "SELL"):
		return ansiRed
	default:
		return ""
	}
}

// ─── HTML Rendering ──────────────────────────────────────────────────────────

func renderMarketHTML(results map[string]*MarketResult, view []SectionConfig, viewName string, showMovers bool, now time.Time) string {
	report := HTMLReport{
		Title:     "Market Overview",
		Date:      now.Format("Monday, 02 Jan 2006"),
		Generated: now.Format("15:04:05 SGT, 02 Jan 2006"),
		ViewName:  viewName,
		HasMovers: showMovers,
	}

	for _, sec := range view {
		hs := HTMLSection{Name: sec.Name}
		for _, t := range sec.Tickers {
			mr, ok := results[t.Symbol]
			if !ok {
				continue
			}
			row := HTMLRow{
				Name:         mr.Name,
				Price:        fmtPrice(mr.Price),
				ChangePct:    mr.ChangePct,
				ChangePctStr: fmt.Sprintf("%+.1f%%", mr.ChangePct),
				ChangeArrow:  changeArrow(mr.ChangePct),
				TrendPct:     effectiveMoverPct(mr),
			}
			if mr.Week52High > 0 {
				row.Week52 = fmt.Sprintf("%.0f-%.0f", mr.Week52Low, mr.Week52High)
			}
			if mr.PETrailing > 0 {
				row.PETrailing = fmt.Sprintf("%.1f", mr.PETrailing)
			}
			if mr.PEForward > 0 {
				row.PEForward = fmt.Sprintf("%.1f", mr.PEForward)
			}
			for _, p := range historicalPeriods {
				val, exists := mr.Historical[p.Label]
				cell := HTMLHistCell{Label: p.Label}
				if exists {
					cell.Value = fmt.Sprintf("%+.1f%%", val)
					cell.Class = htmlColorClass(val)
				} else {
					cell.Value = "-"
					cell.Class = "neutral"
				}
				row.Historical = append(row.Historical, cell)
			}
			hs.Rows = append(hs.Rows, row)
		}
		report.Sections = append(report.Sections, hs)
	}

	if showMovers {
		for _, sec := range topMoversStocks {
			ms := HTMLMoversSection{Region: sec.Name}
			type mover struct {
				name      string
				changePct float64
			}
			var movers []mover
			for _, t := range sec.Tickers {
				if mr, ok := results[t.Symbol]; ok {
					movers = append(movers, mover{mr.Name, effectiveMoverPct(mr)})
				}
			}
			sort.Slice(movers, func(i, j int) bool { return movers[i].changePct > movers[j].changePct })
			top := 5
			if len(movers) < top {
				top = len(movers)
			}
			for i := 0; i < top; i++ {
				ms.Gainers = append(ms.Gainers, HTMLMover{
					Name:      movers[i].name,
					ChangePct: fmt.Sprintf("%+.1f%%", movers[i].changePct),
					Class:     htmlColorClass(movers[i].changePct),
				})
			}
			for i := len(movers) - 1; i >= len(movers)-top && i >= 0; i-- {
				ms.Losers = append(ms.Losers, HTMLMover{
					Name:      movers[i].name,
					ChangePct: fmt.Sprintf("%+.1f%%", movers[i].changePct),
					Class:     htmlColorClass(movers[i].changePct),
				})
			}
			report.Movers = append(report.Movers, ms)
		}
	}

	funcMap := template.FuncMap{
		"colorClass": htmlColorClass,
		"trendArrow": htmlTrendArrow,
	}
	tmpl, err := template.New("market").Funcs(funcMap).Parse(embeddedTemplate)
	if err != nil {
		log.Printf("Template parse error: %v", err)
		return "<html><body>Template error</body></html>"
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, report); err != nil {
		log.Printf("Template exec error: %v", err)
		return "<html><body>Template error</body></html>"
	}
	return buf.String()
}

func renderStockDetailHTML(q YFQuote, summary *YFSummaryResult, now time.Time) string {
	name := q.LongName
	if name == "" {
		name = q.ShortName
	}
	data := map[string]interface{}{
		"Symbol":    q.Symbol,
		"Name":      name,
		"Currency":  q.Currency,
		"Price":     fmtPrice(q.RegularMarketPrice),
		"Change":    fmt.Sprintf("%+.2f", q.RegularMarketChange),
		"ChangePct": fmt.Sprintf("%+.2f%%", q.RegularMarketChangePct),
		"Open":      fmtPrice(q.RegularMarketOpen),
		"DayLow":    fmtPrice(q.RegularMarketDayLow),
		"DayHigh":   fmtPrice(q.RegularMarketDayHigh),
		"W52Low":    fmtPrice(q.FiftyTwoWeekLow),
		"W52High":   fmtPrice(q.FiftyTwoWeekHigh),
		"Volume":    addCommas(fmt.Sprintf("%d", q.RegularMarketVolume)),
		"AvgVolume": addCommas(fmt.Sprintf("%d", q.AvgVolume3M)),
		"MarketCap": fmtLargeNum(q.MarketCap),
		"PE":        fmt.Sprintf("%.2f / %.2f", q.TrailingPE, q.ForwardPE),
		"PB":        fmt.Sprintf("%.2f", q.PriceToBook),
		"EPS":       fmt.Sprintf("%.2f / %.2f", q.EPSTrailing, q.EPSForward),
		"Date":      now.Format("Monday, 02 Jan 2006"),
		"Generated": now.Format("15:04:05 SGT"),
		"IsUp":      q.RegularMarketChangePct >= 0,
	}
	if summary != nil && summary.AssetProfile != nil {
		data["Sector"] = summary.AssetProfile.Sector
		data["Industry"] = summary.AssetProfile.Industry
	}
	if summary != nil && summary.FinancialData != nil {
		fd := summary.FinancialData
		data["Rec"] = strings.ToUpper(fd.RecommendationKey)
		if fd.TargetMeanPrice != nil {
			data["TargetMean"] = fmt.Sprintf("%.2f", fd.TargetMeanPrice.Raw)
			upside := ((fd.TargetMeanPrice.Raw - q.RegularMarketPrice) / q.RegularMarketPrice) * 100
			data["Upside"] = fmt.Sprintf("%+.1f%%", upside)
		}
	}

	const stockTmpl = `<!DOCTYPE html><html><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"><title>{{.Symbol}}</title>
<style>body{font-family:'Segoe UI',Tahoma,sans-serif;background:#f4f6f9;margin:0;padding:20px;color:#2d3436}.c{max-width:680px;margin:0 auto;background:#fff;border-radius:12px;box-shadow:0 2px 12px rgba(0,0,0,.08);overflow:hidden}.h{background:linear-gradient(135deg,#0c2461 0%,#1e3799 50%,#0a3d62 100%);color:#fff;padding:32px 28px;text-align:center}.h h1{margin:0 0 4px;font-size:24px}.h .sub{font-size:14px;opacity:.85}.b{padding:24px 28px}.r{display:flex;justify-content:space-between;padding:10px 0;border-bottom:1px solid #f0f2f5}.r .l{color:#636e72;font-size:13px}.r .v{font-weight:600;font-size:13px}.up{color:#22c55e}.dn{color:#ef4444}.sec{font-weight:700;color:#0c2461;padding:16px 0 8px;border-bottom:2px solid #1e3799;font-size:15px}.f{background:#f8f9fa;padding:16px 20px;font-size:12px;color:#636e72;border-top:1px solid #eee;text-align:center}</style></head>
<body><div class="c"><div class="h"><h1>{{.Symbol}} — {{.Name}}</h1><div class="sub">{{.Date}} (SGT) · {{.Currency}}</div></div>
<div class="b"><div class="r"><span class="l">Price</span><span class="v {{if .IsUp}}up{{else}}dn{{end}}">{{.Price}} ({{.Change}} / {{.ChangePct}})</span></div>
<div class="r"><span class="l">Open</span><span class="v">{{.Open}}</span></div>
<div class="r"><span class="l">Day Range</span><span class="v">{{.DayLow}} — {{.DayHigh}}</span></div>
<div class="r"><span class="l">52W Range</span><span class="v">{{.W52Low}} — {{.W52High}}</span></div>
<div class="r"><span class="l">Volume</span><span class="v">{{.Volume}} (Avg: {{.AvgVolume}})</span></div>
<div class="r"><span class="l">Market Cap</span><span class="v">{{.MarketCap}}</span></div>
{{if .Sector}}<div class="r"><span class="l">Sector</span><span class="v">{{.Sector}}</span></div>{{end}}
{{if .Industry}}<div class="r"><span class="l">Industry</span><span class="v">{{.Industry}}</span></div>{{end}}
<div class="sec">Valuation</div>
<div class="r"><span class="l">PE (T/F)</span><span class="v">{{.PE}}</span></div>
<div class="r"><span class="l">P/B</span><span class="v">{{.PB}}</span></div>
<div class="r"><span class="l">EPS (T/F)</span><span class="v">{{.EPS}}</span></div>
{{if .Rec}}<div class="sec">Analyst</div>
<div class="r"><span class="l">Recommendation</span><span class="v">{{.Rec}}</span></div>
{{if .TargetMean}}<div class="r"><span class="l">Mean Target</span><span class="v">{{.TargetMean}} ({{.Upside}})</span></div>{{end}}{{end}}
</div><div class="f">Generated: {{.Generated}} | Claude Skills - Market Overview</div></div></body></html>`

	tmpl, _ := template.New("stock").Parse(stockTmpl)
	var buf bytes.Buffer
	tmpl.Execute(&buf, data)
	return buf.String()
}

func htmlColorClass(val float64) string {
	if val > 0 {
		return "positive"
	} else if val < 0 {
		return "negative"
	}
	return "neutral"
}

func htmlTrendArrow(val float64) template.HTML {
	var arrow, color string
	switch {
	case val > 1.5:
		arrow, color = "▲▲", "#22c55e"
	case val > 0:
		arrow, color = "▲", "#22c55e"
	case val < -1.5:
		arrow, color = "▼▼", "#ef4444"
	case val < 0:
		arrow, color = "▼", "#ef4444"
	default:
		arrow, color = "━", "#9ca3af"
	}
	return template.HTML(fmt.Sprintf(
		`<span style="font-size:14px;font-weight:700;color:%s">%s</span>`,
		color, arrow))
}

// effectiveMoverPct returns the best available period for sorting/displaying movers.
// Falls back to 1W, 1M, 3M when 1D is flat (e.g. markets are closed).
func effectiveMoverPct(mr *MarketResult) float64 {
	if math.Abs(mr.ChangePct) > 0.01 {
		return mr.ChangePct
	}
	for _, p := range []string{"1W", "1M", "3M"} {
		if v, ok := mr.Historical[p]; ok && math.Abs(v) > 0.01 {
			return v
		}
	}
	return mr.ChangePct
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

func saveFallback(htmlBody string, now time.Time, prefix string) {
	path := fmt.Sprintf("%s_%s.html", prefix, now.Format("20060102_150405"))
	if err := os.WriteFile(path, []byte(htmlBody), 0644); err != nil {
		log.Printf("Failed to save fallback: %v", err)
	} else {
		log.Printf("Report saved to: %s", path)
	}
}

// unused but keep for future
var _ = strconv.Itoa
