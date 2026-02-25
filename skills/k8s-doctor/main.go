package main

import (
	_ "embed"
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/smtp"
	"os"
	"os/exec"
	"sort"
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

// ─── Kubernetes API types ─────────────────────────────────────────────────────

type NodeList struct {
	Items []Node `json:"items"`
}

type Node struct {
	Metadata struct {
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	} `json:"metadata"`
	Status struct {
		Conditions []NodeCondition `json:"conditions"`
	} `json:"status"`
}

type NodeCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type PodList struct {
	Items []Pod `json:"items"`
}

type Pod struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		NodeName string `json:"nodeName"`
	} `json:"spec"`
	Status struct {
		Phase             string            `json:"phase"`
		Conditions        []PodCondition    `json:"conditions"`
		ContainerStatuses []ContainerStatus `json:"containerStatuses"`
	} `json:"status"`
}

type PodCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type ContainerStatus struct {
	Name         string         `json:"name"`
	Ready        bool           `json:"ready"`
	RestartCount int            `json:"restartCount"`
	State        ContainerState `json:"state"`
	LastState    ContainerState `json:"lastState"`
}

type ContainerState struct {
	Running *struct {
		StartedAt string `json:"startedAt"`
	} `json:"running"`
	Waiting *struct {
		Reason  string `json:"reason"`
		Message string `json:"message"`
	} `json:"waiting"`
	Terminated *struct {
		Reason     string `json:"reason"`
		ExitCode   int    `json:"exitCode"`
		FinishedAt string `json:"finishedAt"`
	} `json:"terminated"`
}

type DeploymentList struct {
	Items []Deployment `json:"items"`
}

type Deployment struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Replicas int `json:"replicas"`
	} `json:"spec"`
	Status struct {
		Replicas            int               `json:"replicas"`
		ReadyReplicas       int               `json:"readyReplicas"`
		AvailableReplicas   int               `json:"availableReplicas"`
		UnavailableReplicas int               `json:"unavailableReplicas"`
		UpdatedReplicas     int               `json:"updatedReplicas"`
		Conditions          []DeployCondition `json:"conditions"`
	} `json:"status"`
}

type DeployCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type EventList struct {
	Items []Event `json:"items"`
}

type Event struct {
	Metadata struct {
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	InvolvedObject struct {
		Kind      string `json:"kind"`
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"involvedObject"`
	Reason        string `json:"reason"`
	Message       string `json:"message"`
	Type          string `json:"type"`
	Count         int    `json:"count"`
	LastTimestamp string `json:"lastTimestamp"`
}

// ─── Report types ─────────────────────────────────────────────────────────────

type Severity int

const (
	SeverityCritical Severity = iota
	SeverityWarning
	SeverityInfo
)

type Issue struct {
	Severity  Severity
	Kind      string
	Namespace string
	Resource  string
	Message   string
	Details   string
}

type NodeSummary struct {
	Name   string
	Status string
	Issues []string
}

type PodIssue struct {
	Namespace string
	Name      string
	State     string
	Restarts  int
	Node      string
}

type DeployIssue struct {
	Namespace string
	Name      string
	Desired   int
	Ready     int
	Issue     string
}

type EventSummary struct {
	Namespace string
	Kind      string
	Name      string
	Reason    string
	Message   string
	Count     int
}

type Report struct {
	GeneratedAt    string
	ClusterContext string
	HealthScore    int
	TotalIssues    int
	Critical       int
	Warnings       int
	NodeCount      int
	PodCount       int
	Issues         []Issue
	NodeSummary    []NodeSummary
	PodIssues      []PodIssue
	DeployIssues   []DeployIssue
	RecentEvents   []EventSummary
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

func kubectl(args ...string) ([]byte, error) {
	cmd := exec.Command("kubectl", args...)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(errBuf.String()))
	}
	return out.Bytes(), nil
}

func getContext() string {
	b, err := kubectl("config", "current-context")
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(b))
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ─── Checks ───────────────────────────────────────────────────────────────────

func checkNodes() ([]NodeSummary, []Issue, int) {
	data, err := kubectl("get", "nodes", "-o", "json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: get nodes: %v\n", err)
		return nil, nil, 0
	}
	var list NodeList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, nil, 0
	}
	var summaries []NodeSummary
	var issues []Issue
	for _, n := range list.Items {
		s := NodeSummary{Name: n.Metadata.Name, Status: "Ready"}
		var ni []string
		for _, c := range n.Status.Conditions {
			switch c.Type {
			case "Ready":
				if c.Status != "True" {
					s.Status = "NotReady"
					ni = append(ni, "not-ready:"+c.Reason)
					issues = append(issues, Issue{Severity: SeverityCritical, Kind: "Node", Resource: n.Metadata.Name, Message: "Node NotReady", Details: c.Reason})
				}
			case "MemoryPressure":
				if c.Status == "True" {
					ni = append(ni, "memory-pressure")
					issues = append(issues, Issue{Severity: SeverityWarning, Kind: "Node", Resource: n.Metadata.Name, Message: "MemoryPressure"})
				}
			case "DiskPressure":
				if c.Status == "True" {
					ni = append(ni, "disk-pressure")
					issues = append(issues, Issue{Severity: SeverityWarning, Kind: "Node", Resource: n.Metadata.Name, Message: "DiskPressure"})
				}
			case "PIDPressure":
				if c.Status == "True" {
					ni = append(ni, "pid-pressure")
					issues = append(issues, Issue{Severity: SeverityWarning, Kind: "Node", Resource: n.Metadata.Name, Message: "PIDPressure"})
				}
			}
		}
		s.Issues = ni
		summaries = append(summaries, s)
	}
	return summaries, issues, len(list.Items)
}

func checkPods(ns string, allNS bool) ([]PodIssue, []Issue, int) {
	args := []string{"get", "pods", "-o", "json"}
	if allNS {
		args = append(args, "--all-namespaces")
	} else if ns != "" {
		args = append(args, "-n", ns)
	}
	data, err := kubectl(args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: get pods: %v\n", err)
		return nil, nil, 0
	}
	var list PodList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, nil, 0
	}
	var podIssues []PodIssue
	var issues []Issue
	for _, p := range list.Items {
		if p.Status.Phase == "Succeeded" {
			continue
		}
		for _, cs := range p.Status.ContainerStatuses {
			issueStr := ""
			sev := SeverityWarning
			if cs.State.Waiting != nil {
				switch cs.State.Waiting.Reason {
				case "CrashLoopBackOff":
					issueStr, sev = "CrashLoopBackOff", SeverityCritical
				case "OOMKilled":
					issueStr, sev = "OOMKilled", SeverityCritical
				case "ImagePullBackOff", "ErrImagePull":
					issueStr = cs.State.Waiting.Reason
				case "CreateContainerConfigError":
					issueStr = "ConfigError"
				}
			}
			if cs.LastState.Terminated != nil && cs.LastState.Terminated.Reason == "OOMKilled" && issueStr == "" {
				issueStr = "OOMKilled(prev)"
			}
			if cs.RestartCount > 10 && issueStr == "" {
				issueStr = fmt.Sprintf("HighRestarts:%d", cs.RestartCount)
			}
			if issueStr != "" {
				podIssues = append(podIssues, PodIssue{
					Namespace: p.Metadata.Namespace, Name: p.Metadata.Name,
					State: issueStr, Restarts: cs.RestartCount, Node: p.Spec.NodeName,
				})
				issues = append(issues, Issue{
					Severity: sev, Kind: "Pod", Namespace: p.Metadata.Namespace, Resource: p.Metadata.Name,
					Message: issueStr, Details: fmt.Sprintf("restarts:%d container:%s", cs.RestartCount, cs.Name),
				})
			}
		}
		if p.Status.Phase == "Pending" {
			reason := "Pending"
			for _, c := range p.Status.Conditions {
				if c.Type == "PodScheduled" && c.Status == "False" {
					reason = c.Reason
				}
			}
			podIssues = append(podIssues, PodIssue{Namespace: p.Metadata.Namespace, Name: p.Metadata.Name, State: "Pending", Node: p.Spec.NodeName})
			issues = append(issues, Issue{Severity: SeverityWarning, Kind: "Pod", Namespace: p.Metadata.Namespace, Resource: p.Metadata.Name, Message: "Pending:" + reason})
		}
		if p.Status.Phase == "Failed" {
			podIssues = append(podIssues, PodIssue{Namespace: p.Metadata.Namespace, Name: p.Metadata.Name, State: "Failed", Node: p.Spec.NodeName})
			issues = append(issues, Issue{Severity: SeverityCritical, Kind: "Pod", Namespace: p.Metadata.Namespace, Resource: p.Metadata.Name, Message: "Pod Failed"})
		}
	}
	return podIssues, issues, len(list.Items)
}

func checkDeployments(ns string, allNS bool) ([]DeployIssue, []Issue) {
	args := []string{"get", "deployments", "-o", "json"}
	if allNS {
		args = append(args, "--all-namespaces")
	} else if ns != "" {
		args = append(args, "-n", ns)
	}
	data, err := kubectl(args...)
	if err != nil {
		return nil, nil
	}
	var list DeploymentList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, nil
	}
	var dis []DeployIssue
	var issues []Issue
	for _, d := range list.Items {
		if d.Status.UnavailableReplicas > 0 {
			msg := fmt.Sprintf("%d/%d unavailable", d.Status.UnavailableReplicas, d.Spec.Replicas)
			dis = append(dis, DeployIssue{Namespace: d.Metadata.Namespace, Name: d.Metadata.Name, Desired: d.Spec.Replicas, Ready: d.Status.ReadyReplicas, Issue: msg})
			issues = append(issues, Issue{Severity: SeverityWarning, Kind: "Deployment", Namespace: d.Metadata.Namespace, Resource: d.Metadata.Name, Message: msg})
		}
		for _, c := range d.Status.Conditions {
			if c.Type == "Progressing" && c.Reason == "ProgressDeadlineExceeded" {
				msg := "Rollout stalled: ProgressDeadlineExceeded"
				dis = append(dis, DeployIssue{Namespace: d.Metadata.Namespace, Name: d.Metadata.Name, Desired: d.Spec.Replicas, Ready: d.Status.ReadyReplicas, Issue: msg})
				issues = append(issues, Issue{Severity: SeverityCritical, Kind: "Deployment", Namespace: d.Metadata.Namespace, Resource: d.Metadata.Name, Message: msg})
			}
		}
	}
	return dis, issues
}

func checkEvents(ns string, allNS bool) ([]EventSummary, []Issue) {
	args := []string{"get", "events", "-o", "json", "--field-selector=type=Warning"}
	if allNS {
		args = append(args, "--all-namespaces")
	} else if ns != "" {
		args = append(args, "-n", ns)
	}
	data, err := kubectl(args...)
	if err != nil {
		return nil, nil
	}
	var list EventList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, nil
	}
	sort.Slice(list.Items, func(i, j int) bool {
		return list.Items[i].Count > list.Items[j].Count
	})
	var summaries []EventSummary
	var issues []Issue
	for _, e := range list.Items {
		summaries = append(summaries, EventSummary{
			Namespace: e.Metadata.Namespace, Kind: e.InvolvedObject.Kind,
			Name: e.InvolvedObject.Name, Reason: e.Reason,
			Message: trunc(e.Message, 100), Count: e.Count,
		})
		if e.Count > 5 {
			issues = append(issues, Issue{
				Severity: SeverityWarning, Kind: e.InvolvedObject.Kind,
				Namespace: e.Metadata.Namespace, Resource: e.InvolvedObject.Name,
				Message: e.Reason + ": " + trunc(e.Message, 80), Details: fmt.Sprintf("count:%d", e.Count),
			})
		}
	}
	if len(summaries) > 20 {
		summaries = summaries[:20]
	}
	return summaries, issues
}

// ─── Score & output ───────────────────────────────────────────────────────────

func healthScore(critical, warnings, nodeCount int) int {
	if nodeCount == 0 {
		return 0
	}
	s := 100 - critical*15 - warnings*5
	if s < 0 {
		return 0
	}
	return s
}

func printReport(r *Report) {
	fmt.Printf("%s%sK8s Doctor — Cluster Health Report%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("Context  : %s\n", r.ClusterContext)
	fmt.Printf("Generated: %s\n\n", r.GeneratedAt)

	sc := colorGreen
	if r.HealthScore < 70 {
		sc = colorYellow
	}
	if r.HealthScore < 40 {
		sc = colorRed
	}
	fmt.Printf("%s%sHealth: %d/100%s  Critical: %s%d%s  Warnings: %s%d%s  Nodes: %d  Pods: %d\n\n",
		colorBold, sc, r.HealthScore, colorReset,
		colorRed, r.Critical, colorReset,
		colorYellow, r.Warnings, colorReset,
		r.NodeCount, r.PodCount)

	if len(r.Issues) == 0 {
		fmt.Printf("%s✓ No issues found — cluster looks healthy!%s\n", colorGreen, colorReset)
		return
	}

	// Nodes
	fmt.Printf("%s%s=== Nodes ===%s\n", colorBold, colorBlue, colorReset)
	fmt.Printf("%-32s %-12s %s\n", "NAME", "STATUS", "CONDITIONS")
	fmt.Println(strings.Repeat("─", 80))
	for _, n := range r.NodeSummary {
		sc := colorGreen
		if n.Status != "Ready" {
			sc = colorRed
		}
		issues := strings.Join(n.Issues, " | ")
		if issues == "" {
			issues = "—"
		}
		fmt.Printf("%-32s %s%-12s%s %s\n", n.Name, sc, n.Status, colorReset, issues)
	}

	// Pod issues
	if len(r.PodIssues) > 0 {
		fmt.Printf("\n%s%s=== Pod Issues ===%s\n", colorBold, colorBlue, colorReset)
		fmt.Printf("%-20s %-40s %-22s %-8s %s\n", "NAMESPACE", "POD", "STATE", "RESTARTS", "NODE")
		fmt.Println(strings.Repeat("─", 105))
		for _, p := range r.PodIssues {
			sc := colorYellow
			switch p.State {
			case "CrashLoopBackOff", "OOMKilled", "Failed":
				sc = colorRed
			}
			fmt.Printf("%-20s %-40s %s%-22s%s %-8d %s\n",
				trunc(p.Namespace, 20), trunc(p.Name, 40), sc, p.State, colorReset, p.Restarts, p.Node)
		}
	}

	// Deployment issues
	if len(r.DeployIssues) > 0 {
		fmt.Printf("\n%s%s=== Deployment Issues ===%s\n", colorBold, colorBlue, colorReset)
		fmt.Printf("%-20s %-40s %-8s %-8s %s\n", "NAMESPACE", "DEPLOYMENT", "DESIRED", "READY", "ISSUE")
		fmt.Println(strings.Repeat("─", 105))
		for _, d := range r.DeployIssues {
			fmt.Printf("%-20s %-40s %-8d %-8d %s%s%s\n",
				trunc(d.Namespace, 20), trunc(d.Name, 40), d.Desired, d.Ready, colorYellow, d.Issue, colorReset)
		}
	}

	// Events
	if len(r.RecentEvents) > 0 {
		fmt.Printf("\n%s%s=== Warning Events (top 20) ===%s\n", colorBold, colorBlue, colorReset)
		fmt.Printf("%-15s %-10s %-28s %-22s %-6s %s\n", "NAMESPACE", "KIND", "NAME", "REASON", "COUNT", "MESSAGE")
		fmt.Println(strings.Repeat("─", 115))
		for _, e := range r.RecentEvents {
			fmt.Printf("%-15s %-10s %-28s %-22s %-6d %s\n",
				trunc(e.Namespace, 15), trunc(e.Kind, 10), trunc(e.Name, 28),
				trunc(e.Reason, 22), e.Count, trunc(e.Message, 50))
		}
	}

	// Issues summary
	fmt.Printf("\n%s%s=== All Issues ===%s\n", colorBold, colorBlue, colorReset)
	for _, issue := range r.Issues {
		c, label := colorYellow, "WARN"
		if issue.Severity == SeverityCritical {
			c, label = colorRed, "CRIT"
		}
		ns := issue.Namespace
		if ns == "" {
			ns = "cluster"
		}
		det := ""
		if issue.Details != "" {
			det = " (" + issue.Details + ")"
		}
		fmt.Printf("%s[%s]%s %s/%s (%s): %s%s\n", c, label, colorReset, issue.Kind, issue.Resource, ns, issue.Message, det)
	}
}

func saveHTML(r *Report, filename string) error {
	tmpl, err := template.New("k8s").Parse(emailTemplate)
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
	rcpt := os.Getenv("K8S_RECIPIENTS")
	if user == "" || pass == "" || rcpt == "" {
		return fmt.Errorf("set GMAIL_USER, GMAIL_APP_PASSWORD, K8S_RECIPIENTS in .env")
	}
	tmpl, err := template.New("k8s").Parse(emailTemplate)
	if err != nil {
		return err
	}
	var body bytes.Buffer
	if err := tmpl.Execute(&body, r); err != nil {
		return err
	}
	to := strings.Split(rcpt, ",")
	subj := fmt.Sprintf("K8s Health: %d/100 | Critical: %d | Warnings: %d | %s",
		r.HealthScore, r.Critical, r.Warnings, r.ClusterContext)
	msg := "MIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n"
	msg += "From: " + user + "\r\nTo: " + strings.Join(to, ", ") + "\r\nSubject: " + subj + "\r\n\r\n"
	msg += body.String()
	auth := smtp.PlainAuth("", user, pass, "smtp.gmail.com")
	return smtp.SendMail("smtp.gmail.com:587", auth, user, to, []byte(msg))
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	ns := flag.String("n", "", "Namespace to check (default: all)")
	allNS := flag.Bool("a", true, "Check all namespaces")
	htmlOut := flag.String("html", "", "Save HTML report to file")
	doEmail := flag.Bool("email", false, "Send email report")
	flag.Parse()

	loadEnv()

	if *ns != "" {
		*allNS = false
	}

	fmt.Printf("%sK8s Doctor%s — Kubernetes Cluster Health\n", colorBold, colorReset)
	fmt.Println("Analyzing cluster…")

	r := &Report{
		GeneratedAt:    time.Now().Format("2006-01-02 15:04:05 MST"),
		ClusterContext: getContext(),
	}

	nodeSummaries, nodeIssues, nodeCount := checkNodes()
	podIssues, podIssueList, podCount := checkPods(*ns, *allNS)
	deployIssues, deployIssueList := checkDeployments(*ns, *allNS)
	eventSummaries, eventIssues := checkEvents(*ns, *allNS)

	all := append(nodeIssues, podIssueList...)
	all = append(all, deployIssueList...)
	all = append(all, eventIssues...)
	sort.Slice(all, func(i, j int) bool { return all[i].Severity < all[j].Severity })

	crit, warn := 0, 0
	for _, i := range all {
		if i.Severity == SeverityCritical {
			crit++
		} else if i.Severity == SeverityWarning {
			warn++
		}
	}

	r.Issues = all
	r.NodeSummary = nodeSummaries
	r.PodIssues = podIssues
	r.DeployIssues = deployIssues
	r.RecentEvents = eventSummaries
	r.NodeCount = nodeCount
	r.PodCount = podCount
	r.Critical = crit
	r.Warnings = warn
	r.TotalIssues = len(all)
	r.HealthScore = healthScore(crit, warn, nodeCount)

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
			fn := fmt.Sprintf("k8s-doctor-%s.html", time.Now().Format("20060102-150405"))
			if saveHTML(r, fn) == nil {
				fmt.Printf("\n%sEmail failed — saved: %s%s\n", colorYellow, fn, colorReset)
			}
			fmt.Fprintf(os.Stderr, "email error: %v\n", err)
		} else {
			fmt.Printf(" %s✓ Sent%s\n", colorGreen, colorReset)
		}
	}
}
