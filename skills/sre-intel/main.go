package main

import (
	_ "embed"
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"math"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed template.html
var emailTemplate string

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// ─── Prometheus response types ────────────────────────────────────────────────

type PromResponse struct {
	Status string   `json:"status"`
	Data   PromData `json:"data"`
	Error  string   `json:"error"`
}

type PromData struct {
	ResultType string       `json:"resultType"`
	Result     []PromResult `json:"result"`
}

type PromResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"` // [timestamp, "value_string"]
}

// ─── Alertmanager response types ──────────────────────────────────────────────

type AMResponse struct {
	Status string    `json:"status"`
	Data   []AMAlert `json:"data"`
}

type AMAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	State       string            `json:"state"`
	StartsAt    string            `json:"startsAt"`
	EndsAt      string            `json:"endsAt"`
}

// ─── Report types ─────────────────────────────────────────────────────────────

type FiringAlert struct {
	Name        string
	Severity    string
	State       string
	Summary     string
	Description string
	StartsAt    string
	Labels      map[string]string
}

type ServiceMetric struct {
	Job          string
	ErrorRate    float64
	RequestRate  float64
	LatencyP50   float64
	LatencyP95   float64
	LatencyP99   float64
	Availability float64
	ErrorBudget  float64 // remaining % of monthly error budget
	BurnRate     string  // slow/fast/critical
}

type Report struct {
	GeneratedAt     string
	PrometheusURL   string
	AlertmanagerURL string
	Range           string
	SLOTarget       float64
	FiringAlerts    []FiringAlert
	CriticalAlerts  int
	WarningAlerts   int
	Services        []ServiceMetric
	TotalServices   int
	HealthyServices int
	DegradedServices int
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func loadEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
}

func httpGet(rawURL string) ([]byte, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func promQuery(baseURL, query string) ([]PromResult, error) {
	endpoint := baseURL + "/api/v1/query"
	params := url.Values{}
	params.Set("query", query)
	fullURL := endpoint + "?" + params.Encode()

	body, err := httpGet(fullURL)
	if err != nil {
		return nil, err
	}
	var resp PromResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("prometheus error: %s", resp.Error)
	}
	return resp.Data.Result, nil
}

func resultValue(r PromResult) float64 {
	if len(r.Value) < 2 {
		return 0
	}
	s, ok := r.Value[1].(string)
	if !ok {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func jobLabel(r PromResult) string {
	if j, ok := r.Metric["job"]; ok && j != "" {
		return j
	}
	if s, ok := r.Metric["service"]; ok && s != "" {
		return s
	}
	for k, v := range r.Metric {
		if k != "__name__" {
			return k + "=" + v
		}
	}
	return "unknown"
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ─── Prometheus queries ───────────────────────────────────────────────────────

func fetchAlerts(amURL string) ([]FiringAlert, error) {
	body, err := httpGet(amURL + "/api/v2/alerts")
	if err != nil {
		// try v1 fallback
		body, err = httpGet(amURL + "/api/v1/alerts")
		if err != nil {
			return nil, err
		}
		// v1 wraps in {"status":"success","data":[...]}
		var v1 AMResponse
		if err2 := json.Unmarshal(body, &v1); err2 == nil && v1.Status == "success" {
			return convertAlerts(v1.Data), nil
		}
	}
	// v2 returns raw array
	var alerts []AMAlert
	if err := json.Unmarshal(body, &alerts); err != nil {
		// maybe v1 format
		var v1 AMResponse
		if err2 := json.Unmarshal(body, &v1); err2 == nil {
			return convertAlerts(v1.Data), nil
		}
		return nil, err
	}
	return convertAlerts(alerts), nil
}

func convertAlerts(raw []AMAlert) []FiringAlert {
	var out []FiringAlert
	for _, a := range raw {
		fa := FiringAlert{
			Name:        a.Labels["alertname"],
			Severity:    a.Labels["severity"],
			State:       a.State,
			Summary:     a.Annotations["summary"],
			Description: a.Annotations["description"],
			StartsAt:    a.StartsAt,
			Labels:      a.Labels,
		}
		if fa.Name == "" {
			fa.Name = "unknown"
		}
		if fa.Severity == "" {
			fa.Severity = "unknown"
		}
		out = append(out, fa)
	}
	// sort: critical first
	sort.Slice(out, func(i, j int) bool {
		order := map[string]int{"critical": 0, "warning": 1, "info": 2}
		oi, oj := 3, 3
		if v, ok := order[out[i].Severity]; ok {
			oi = v
		}
		if v, ok := order[out[j].Severity]; ok {
			oj = v
		}
		return oi < oj
	})
	return out
}

func fetchErrorRates(promURL, rangeStr, jobFilter string) (map[string]float64, error) {
	q := fmt.Sprintf(`100 * sum(rate(http_requests_total{status=~"5.."}[%s])) by (job) / sum(rate(http_requests_total[%s])) by (job)`, rangeStr, rangeStr)
	results, err := promQuery(promURL, q)
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, r := range results {
		job := jobLabel(r)
		if jobFilter != "" && !strings.Contains(strings.ToLower(job), strings.ToLower(jobFilter)) {
			continue
		}
		out[job] = resultValue(r)
	}
	return out, nil
}

func fetchRequestRates(promURL, rangeStr, jobFilter string) (map[string]float64, error) {
	q := fmt.Sprintf(`sum(rate(http_requests_total[%s])) by (job)`, rangeStr)
	results, err := promQuery(promURL, q)
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, r := range results {
		job := jobLabel(r)
		if jobFilter != "" && !strings.Contains(strings.ToLower(job), strings.ToLower(jobFilter)) {
			continue
		}
		out[job] = resultValue(r)
	}
	return out, nil
}

func fetchLatency(promURL, rangeStr, quantile, jobFilter string) (map[string]float64, error) {
	q := fmt.Sprintf(
		`histogram_quantile(%s, sum(rate(http_request_duration_seconds_bucket[%s])) by (le, job))`,
		quantile, rangeStr,
	)
	results, err := promQuery(promURL, q)
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, r := range results {
		job := jobLabel(r)
		if jobFilter != "" && !strings.Contains(strings.ToLower(job), strings.ToLower(jobFilter)) {
			continue
		}
		out[job] = resultValue(r)
	}
	return out, nil
}

func fetchAvailability(promURL, jobFilter string) (map[string]float64, error) {
	q := `100 * (1 - sum(rate(http_requests_total{status=~"5.."}[30d])) by (job) / sum(rate(http_requests_total[30d])) by (job))`
	results, err := promQuery(promURL, q)
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, r := range results {
		job := jobLabel(r)
		if jobFilter != "" && !strings.Contains(strings.ToLower(job), strings.ToLower(jobFilter)) {
			continue
		}
		v := resultValue(r)
		if v > 100 {
			v = 100
		}
		out[job] = v
	}
	return out, nil
}

func calcErrorBudget(availability, sloTarget float64) float64 {
	// Error budget remaining = (current_availability - slo_target) / (100 - slo_target) * 100
	allowed := 100 - sloTarget
	if allowed <= 0 {
		return 0
	}
	actual := 100 - availability
	if actual < 0 {
		actual = 0
	}
	remaining := (allowed - actual) / allowed * 100
	if remaining < 0 {
		return 0
	}
	return remaining
}

func burnRateLabel(budget float64) string {
	switch {
	case budget <= 0:
		return "exhausted"
	case budget < 10:
		return "critical"
	case budget < 30:
		return "fast"
	case budget < 70:
		return "moderate"
	default:
		return "healthy"
	}
}

// ─── Terminal output ──────────────────────────────────────────────────────────

func severityColor(sev string) string {
	switch strings.ToLower(sev) {
	case "critical":
		return colorRed
	case "warning":
		return colorYellow
	default:
		return colorCyan
	}
}

func printReport(r *Report) {
	fmt.Printf("%s%sSRE Intel — Observability Dashboard%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("Prometheus  : %s\n", r.PrometheusURL)
	fmt.Printf("Alertmanager: %s\n", r.AlertmanagerURL)
	fmt.Printf("Range       : %s | SLO Target: %.2f%%\n", r.Range, r.SLOTarget)
	fmt.Printf("Generated   : %s\n\n", r.GeneratedAt)

	// Alerts
	fmt.Printf("%s%s=== Firing Alerts ===%s\n", colorBold, colorBlue, colorReset)
	if len(r.FiringAlerts) == 0 {
		fmt.Printf("%s✓ No firing alerts%s\n", colorGreen, colorReset)
	} else {
		fmt.Printf("Critical: %s%d%s  Warning: %s%d%s  Total: %d\n\n",
			colorRed, r.CriticalAlerts, colorReset, colorYellow, r.WarningAlerts, colorReset, len(r.FiringAlerts))
		fmt.Printf("%-12s %-10s %-60s %s\n", "SEVERITY", "STATE", "ALERT", "SINCE")
		fmt.Println(strings.Repeat("─", 100))
		for _, a := range r.FiringAlerts {
			sc := severityColor(a.Severity)
			summary := a.Summary
			if summary == "" {
				summary = a.Name
			}
			since := trunc(a.StartsAt, 20)
			fmt.Printf("%s%-12s%s %-10s %-60s %s\n", sc, a.Severity, colorReset, a.State, trunc(summary, 60), since)
		}
	}

	// Services
	if len(r.Services) > 0 {
		fmt.Printf("\n%s%s=== Service SLIs & SLOs ===%s\n", colorBold, colorBlue, colorReset)
		fmt.Printf("Healthy: %s%d%s  Degraded: %s%d%s  Total: %d\n\n",
			colorGreen, r.HealthyServices, colorReset, colorRed, r.DegradedServices, colorReset, r.TotalServices)
		fmt.Printf("%-30s %-10s %-12s %-8s %-8s %-8s %-12s %-10s %s\n",
			"SERVICE", "ERR%", "REQ/S", "P50ms", "P95ms", "P99ms", "AVAIL(30d)", "BUDGET", "BURN")
		fmt.Println(strings.Repeat("─", 120))
		for _, s := range r.Services {
			ec := colorGreen
			if s.ErrorRate > 5 {
				ec = colorRed
			} else if s.ErrorRate > 1 {
				ec = colorYellow
			}
			bc := colorGreen
			switch s.BurnRate {
			case "exhausted", "critical":
				bc = colorRed
			case "fast":
				bc = colorYellow
			}
			fmt.Printf("%-30s %s%-10.2f%s %-12.1f %-8.0f %-8.0f %-8.0f %-12.3f%% %s%-10.1f%%%s %s%s%s\n",
				trunc(s.Job, 30), ec, s.ErrorRate, colorReset, s.RequestRate,
				s.LatencyP50*1000, s.LatencyP95*1000, s.LatencyP99*1000,
				s.Availability, bc, s.ErrorBudget, colorReset, bc, s.BurnRate, colorReset)
		}
	} else {
		fmt.Printf("\n%sNo service metrics found — check PROMETHEUS_URL and ensure http_requests_total metrics exist%s\n", colorYellow, colorReset)
	}
}

// ─── HTML & email ─────────────────────────────────────────────────────────────

func saveHTML(r *Report, filename string) error {
	tmpl, err := template.New("sre").Funcs(template.FuncMap{
		"mul100": func(f float64) string { return fmt.Sprintf("%.1f", f*100) },
	}).Parse(emailTemplate)
	if err != nil {
		return err
	}
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.Execute(f, r)
}

func sendEmail(r *Report) error {
	user := os.Getenv("GMAIL_USER")
	pass := os.Getenv("GMAIL_APP_PASSWORD")
	rcpt := os.Getenv("SRE_RECIPIENTS")
	if user == "" || pass == "" || rcpt == "" {
		return fmt.Errorf("set GMAIL_USER, GMAIL_APP_PASSWORD, SRE_RECIPIENTS in .env")
	}
	tmpl, err := template.New("sre").Funcs(template.FuncMap{
		"mul100": func(f float64) string { return fmt.Sprintf("%.1f", f*100) },
	}).Parse(emailTemplate)
	if err != nil {
		return err
	}
	var body bytes.Buffer
	if err := tmpl.Execute(&body, r); err != nil {
		return err
	}
	to := strings.Split(rcpt, ",")
	subj := fmt.Sprintf("SRE Intel: %d firing alerts | %d degraded | %d healthy services",
		len(r.FiringAlerts), r.DegradedServices, r.HealthyServices)
	msg := "MIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n"
	msg += "From: " + user + "\r\nTo: " + strings.Join(to, ", ") + "\r\nSubject: " + subj + "\r\n\r\n"
	msg += body.String()
	auth := smtp.PlainAuth("", user, pass, "smtp.gmail.com")
	return smtp.SendMail("smtp.gmail.com:587", auth, user, to, []byte(msg))
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	promURL := flag.String("u", "", "Prometheus URL (overrides PROMETHEUS_URL env)")
	amURL := flag.String("am", "", "Alertmanager URL (overrides ALERTMANAGER_URL env)")
	jobFilter := flag.String("s", "", "Filter by service/job name")
	rangeStr := flag.String("r", "1h", "Lookback range (e.g. 1h, 6h, 24h)")
	sloTarget := flag.Float64("slo", 99.9, "SLO target percentage")
	htmlOut := flag.String("html", "", "Save HTML report to file")
	doEmail := flag.Bool("email", false, "Send email report")
	flag.Parse()

	loadEnv()

	if *promURL == "" {
		*promURL = os.Getenv("PROMETHEUS_URL")
	}
	if *amURL == "" {
		*amURL = os.Getenv("ALERTMANAGER_URL")
	}
	if *promURL == "" {
		*promURL = "http://localhost:9090"
	}
	if *amURL == "" {
		*amURL = "http://localhost:9093"
	}

	fmt.Printf("%sSRE Intel%s — Observability Intelligence\n", colorBold, colorReset)
	fmt.Println("Querying Prometheus and Alertmanager…")

	r := &Report{
		GeneratedAt:     time.Now().Format("2006-01-02 15:04:05 MST"),
		PrometheusURL:   *promURL,
		AlertmanagerURL: *amURL,
		Range:           *rangeStr,
		SLOTarget:       *sloTarget,
	}

	// Fetch alerts
	alerts, err := fetchAlerts(*amURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: alertmanager: %v\n", err)
	} else {
		r.FiringAlerts = alerts
		for _, a := range alerts {
			switch strings.ToLower(a.Severity) {
			case "critical":
				r.CriticalAlerts++
			case "warning":
				r.WarningAlerts++
			}
		}
	}

	// Fetch service metrics
	errRates, _ := fetchErrorRates(*promURL, *rangeStr, *jobFilter)
	reqRates, _ := fetchRequestRates(*promURL, *rangeStr, *jobFilter)
	latP50, _ := fetchLatency(*promURL, *rangeStr, "0.50", *jobFilter)
	latP95, _ := fetchLatency(*promURL, *rangeStr, "0.95", *jobFilter)
	latP99, _ := fetchLatency(*promURL, *rangeStr, "0.99", *jobFilter)
	avail30d, _ := fetchAvailability(*promURL, *jobFilter)

	// Union of all known jobs
	jobs := map[string]struct{}{}
	for j := range errRates {
		jobs[j] = struct{}{}
	}
	for j := range reqRates {
		jobs[j] = struct{}{}
	}
	for j := range avail30d {
		jobs[j] = struct{}{}
	}

	for job := range jobs {
		av := avail30d[job]
		if av == 0 {
			av = 100 // assume 100% if no data
		}
		budget := calcErrorBudget(av, *sloTarget)
		svc := ServiceMetric{
			Job:          job,
			ErrorRate:    errRates[job],
			RequestRate:  reqRates[job],
			LatencyP50:   latP50[job],
			LatencyP95:   latP95[job],
			LatencyP99:   latP99[job],
			Availability: av,
			ErrorBudget:  budget,
			BurnRate:     burnRateLabel(budget),
		}
		r.Services = append(r.Services, svc)
		r.TotalServices++
		if svc.ErrorRate > 1 || svc.BurnRate == "exhausted" || svc.BurnRate == "critical" {
			r.DegradedServices++
		} else {
			r.HealthyServices++
		}
	}

	// Sort by error rate desc
	sort.Slice(r.Services, func(i, j int) bool {
		return r.Services[i].ErrorRate > r.Services[j].ErrorRate
	})

	printReport(r)

	if *htmlOut != "" {
		if err := saveHTML(r, *htmlOut); err != nil {
			fmt.Fprintf(os.Stderr, "HTML error: %v\n", err)
		} else {
			fmt.Printf("\n%s✓ HTML saved: %s%s\n", colorGreen, *htmlOut, colorReset)
		}
	}

	if *doEmail {
		fmt.Print("\nSending email…")
		if err := sendEmail(r); err != nil {
			fn := fmt.Sprintf("sre-intel-%s.html", time.Now().Format("20060102-150405"))
			if saveHTML(r, fn) == nil {
				fmt.Printf("\n%sEmail failed — saved: %s%s\n", colorYellow, fn, colorReset)
			}
			fmt.Fprintf(os.Stderr, "email error: %v\n", err)
		} else {
			fmt.Printf(" %s✓ Sent%s\n", colorGreen, colorReset)
		}
	}
}
