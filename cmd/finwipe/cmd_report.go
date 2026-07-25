package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Dashboard: summary of all requests, outcomes, and compliance metrics",
	Long: `Print a comprehensive dashboard of all deletion requests with:
- Requests by state (pie/bar)
- Requests by category of NBFC
- Overdue requests (past deadline, no response)
- Escalation summary
- Outcome summary
- Average time to acknowledge / close`,
	RunE: runReport,
}

var (
	reportFormat string
	reportDays  int
)

func init() {
	reportCmd.Flags().StringVar(&reportFormat, "format", "terminal",
		"Output format: terminal | json | csv")
	reportCmd.Flags().IntVar(&reportDays, "days", 90,
		"Only show requests created in the last N days")
}

func runReport(cmd *cobra.Command, args []string) error {
	hist, err := history.New(dbPath())
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Load NBFCs for category breakdown
	nbfcPath := filepath.Join(dataDir(), "nbfcs.yaml")
	allNBFCs, _ := nbfc.Load(nbfcPath)
	nbfcMap := make(map[string]nbfc.Entity)
	for _, e := range allNBFCs {
		nbfcMap[e.ID] = e
	}

	// Get all active + recently closed requests
	cutoff := time.Now().AddDate(0, 0, -reportDays)
	all, err := hist.GetAll() // includes closed
	if err != nil {
		return fmt.Errorf("get all requests: %w", err)
	}

	var active, recent []history.Request
	for _, r := range all {
		if r.CreatedAt.After(cutoff) {
			recent = append(recent, r)
		}
		if r.LifecycleState != history.StateClosed {
			active = append(active, r)
		}
	}

	// Build dashboard
	sum, _ := hist.Summary()

	// Count by state
	stateCount := map[history.State]int{}
	escLevelCount := map[int]int{}
	overdueCount := 0
	awaitingAck := 0
	pendingReview := 0

	now := time.Now()
	for _, r := range active {
		stateCount[r.LifecycleState]++
		escLevelCount[r.EscalationLevel]++
		if r.LifecycleState == history.StateDispatched || r.LifecycleState == history.StateInitiated {
			awaitingAck++
			if !r.AckDeadlineAt.IsZero() && now.After(r.AckDeadlineAt) {
				overdueCount++
			}
		}
		if r.LifecycleState == history.StatePendingReview {
			pendingReview++
		}
	}

	// Category breakdown for active requests
	catCount := map[string]int{}
	for _, r := range active {
		cat := "unknown"
		if e, ok := nbfcMap[r.NBFCID]; ok {
			cat = string(e.Category)
		}
		catCount[cat]++
	}

	// Outcomes from closed requests
	closed := []history.Request{}
	for _, r := range all {
		if r.LifecycleState == history.StateClosed {
			closed = append(closed, r)
		}
	}
	outcomeCount := map[string]int{}
	for _, r := range closed {
		outcomeCount[r.Outcome]++
	}

	// Average days to acknowledge
	var ackTimes []float64
	for _, r := range closed {
		if !r.AckReceivedAt.IsZero() {
			ackDays := r.AckReceivedAt.Sub(r.CreatedAt).Hours() / 24
			ackTimes = append(ackTimes, ackDays)
		}
	}

	var avgAckDays float64
	if len(ackTimes) > 0 {
		var sum float64
		for _, d := range ackTimes {
			sum += d
		}
		avgAckDays = sum / float64(len(ackTimes))
	}

	if reportFormat == "json" {
		printJSONReport(sum, stateCount, escLevelCount, catCount, outcomeCount, overdueCount, awaitingAck, pendingReview, avgAckDays)
		return nil
	}

	if reportFormat == "csv" {
		printCSVReport(active, closed, nbfcMap)
		return nil
	}

	// ── Terminal output ─────────────────────────────────────────
	fmt.Printf("\n")
	fmt.Printf("  ╔══════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("  ║         FinWipe — Data Deletion Request Dashboard            ║\n")
	fmt.Printf("  ╠══════════════════════════════════════════════════════════════╣\n")
	fmt.Printf("  ║  Period: last %d days  |  Generated: %s               ║\n",
		reportDays, now.Format("02 Jan 2006 15:04 IST"))
	fmt.Printf("  ╚══════════════════════════════════════════════════════════════╝\n")
	fmt.Println()

	// Active requests summary
	fmt.Printf("  📊 REQUEST SUMMARY\n")
	fmt.Printf("  ──────────────────────────────────────────────────────────────\n")
	fmt.Printf("  %-25s %5d\n", "Total requests created:", len(recent))
	fmt.Printf("  %-25s %5d\n", "Active (non-closed):", len(active))
	fmt.Printf("  %-25s %5d\n", "Closed:", len(closed))
	fmt.Printf("  %-25s %5d\n", "Awaiting acknowledgment:", awaitingAck)
	fmt.Printf("  %-25s %5d\n", "Overdue (past deadline):", overdueCount)
	if pendingReview > 0 {
		fmt.Printf("  %-25s %5d ⚠️  needs your review\n", "Pending your review:", pendingReview)
	}
	fmt.Println()

	// State breakdown
	fmt.Printf("  📍 BY STATE\n")
	fmt.Printf("  ──────────────────────────────────────────────────────────────\n")
	stateOrder := []history.State{
		history.StateInitiated,
		history.StateDispatched,
		history.StateDeliveryFailed,
		history.StateAckReceived,
		history.StatePendingReview,
		history.StateResponseOK,
		history.StateEscalated,
		history.StateClosed,
	}
	for _, s := range stateOrder {
		if count := stateCount[s]; count > 0 {
			label := history.StateLabel(s)
			bar := strings.Repeat("█", count)
			fmt.Printf("  %-25s %5d  %s\n", label, count, bar)
		}
	}
	fmt.Println()

	// Category breakdown
	fmt.Printf("  🏷️  BY NBFC CATEGORY\n")
	fmt.Printf("  ──────────────────────────────────────────────────────────────\n")
	type catPair struct{ cat string; count int }
	var cats []catPair
	for c, n := range catCount {
		cats = append(cats, catPair{c, n})
	}
	sort.Slice(cats, func(i, j int) bool { return cats[i].count > cats[j].count })
	catColors := map[string]string{
		"bank":    "🏦",
		"nbfc":    "🏢",
		"fintech": "💳",
		"hfc":     "🏠",
		"lsp":     "📦",
		"dsp":     "🔗",
	}
	for _, cp := range cats {
		emoji := catColors[cp.cat]
		if emoji == "" {
			emoji = "🏷️"
		}
		fmt.Printf("  %s %-12s %5d  %s\n", emoji, cp.cat, cp.count, strings.Repeat("▌", cp.count))
	}
	fmt.Println()

	// Escalation summary
	fmt.Printf("  🔺 ESCALATION STATUS\n")
	fmt.Printf("  ──────────────────────────────────────────────────────────────\n")
	escLabels := []string{"None (L0)", "DPO (L1)", "DPDP Board (L2)", "RBI/CFPB (L3)", "Consumer (L4)", "Legal (L5)"}
	for lvl := 0; lvl <= 5; lvl++ {
		if count := escLevelCount[lvl]; count > 0 || lvl == 0 {
			bar := strings.Repeat("█", count)
			fmt.Printf("  %-30s %5d  %s\n", escLabels[lvl], count, bar)
		}
	}
	fmt.Println()

	// Outcomes (closed requests)
	if len(closed) > 0 {
		fmt.Printf("  ✅ OUTCOMES (%d closed requests)\n", len(closed))
		fmt.Printf("  ──────────────────────────────────────────────────────────────\n")
		outcomeLabels := map[string]string{
			"deleted":           "✅ Deleted",
			"partial":           "⚠️  Partial",
			"exemption_claimed": "📋 Exemption claimed",
			"no_response":        "❌ No response",
			"rejected":           "🚫 Rejected",
			"escalated":          "🔺 Escalated",
			"withdrawn":         "↩️  Withdrawn",
		}
		for _, o := range sortedKeys(outcomeCount) {
			label := outcomeLabels[o]
			if label == "" {
				label = o
			}
			pct := float64(outcomeCount[o]) / float64(len(closed)) * 100
			bar := strings.Repeat("█", outcomeCount[o])
			fmt.Printf("  %-25s %5d (%5.1f%%)  %s\n", label, outcomeCount[o], pct, bar)
		}
		fmt.Println()
	}

	// Performance metrics
	if len(ackTimes) > 0 {
		fmt.Printf("  ⏱️  PERFORMANCE METRICS\n")
		fmt.Printf("  ──────────────────────────────────────────────────────────────\n")
		fmt.Printf("  %-30s %5.1f days\n", "Avg. time to acknowledge:", avgAckDays)
		fmt.Printf("  %-30s %5d\n", "Success rate (deleted/closed):",
			outcomeCount["deleted"]+outcomeCount["partial"])
		fmt.Println()
	}

	// Overdue list
	if overdueCount > 0 {
		fmt.Printf("  ⚠️  OVERDUE REQUESTS (%d)\n", overdueCount)
		fmt.Printf("  ──────────────────────────────────────────────────────────────\n")
		for _, r := range active {
			if (r.LifecycleState == history.StateDispatched || r.LifecycleState == history.StateInitiated) &&
				!r.AckDeadlineAt.IsZero() && now.After(r.AckDeadlineAt) {
				daysOver := int(now.Sub(r.AckDeadlineAt).Hours() / 24)
				fmt.Printf("  %-20s %-25s %+d days overdue\n",
					r.RequestID, r.NBFCName, daysOver)
			}
			fmt.Println()
		}
	}

	// Next actions
	fmt.Printf("  ▶️  NEXT ACTIONS\n")
	fmt.Printf("  ──────────────────────────────────────────────────────────────\n")
	if awaitingAck > 0 {
		fmt.Printf("  finwipe ack --request-id <ID> --ref <ref>   # Record acknowledgments\n")
	}
	if pendingReview > 0 {
		fmt.Printf("  finwipe review --request-id <ID>            # Review NBFC response\n")
	}
	if overdueCount > 0 {
		fmt.Printf("  finwipe escalate --request-id <ID> --to dpo # Escalate overdue\n")
	}
	if len(active) > 0 {
		fmt.Printf("  finwipe track --all                         # Full detail\n")
	}
	if cfg.SMTP.Password == "" {
		fmt.Println("  ⚠️  SMTP not configured — run: finwipe init")
	}
	fmt.Println()

	return nil
}

func printJSONReport(sum map[string]int, stateCount map[history.State]int,
	escLevelCount map[int]int, catCount map[string]int, outcomeCount map[string]int,
	overdue, awaitingAck, pendingReview int, avgAckDays float64) {
	// Simple JSON without external deps
	type jreport struct {
		TotalCreated  int               `json:"total_created"`
		Active        int               `json:"active"`
		Closed        int               `json:"closed"`
		AwaitingAck   int               `json:"awaiting_ack"`
		Overdue       int               `json:"overdue"`
		PendingReview int               `json:"pending_review"`
		AvgAckDays    float64           `json:"avg_days_to_ack"`
		ByState       map[string]int    `json:"by_state"`
		ByCategory    map[string]int    `json:"by_category"`
		ByEscLevel    map[string]int    `json:"by_escalation_level"`
		ByOutcome     map[string]int    `json:"by_outcome"`
	}
	r := jreport{
		TotalCreated:  sum["total"],
		Active:        sum["total"] - sum["CLOSED"],
		Closed:        sum["CLOSED"],
		AwaitingAck:    awaitingAck,
		Overdue:       overdue,
		PendingReview: pendingReview,
		AvgAckDays:    avgAckDays,
		ByState:       map[string]int{},
		ByCategory:    map[string]int{},
		ByEscLevel:    map[string]int{},
		ByOutcome:     map[string]int{},
	}
	for s, c := range stateCount {
		r.ByState[string(s)] = c
	}
	for c, n := range catCount {
		r.ByCategory[c] = n
	}
	for l, n := range escLevelCount {
		r.ByEscLevel[fmt.Sprintf("L%d", l)] = n
	}
	for o, n := range outcomeCount {
		r.ByOutcome[o] = n
	}
	fmt.Printf("%+v\n", r)
}

func printCSVReport(active, closed []history.Request, nbfcMap map[string]nbfc.Entity) {
	fmt.Println("request_id,nbfc_id,nbfc_name,category,state,escalation_level,created_at,ack_received_at,closed_at,outcome")
	for _, r := range append(active, closed...) {
		cat := "unknown"
		if e, ok := nbfcMap[r.NBFCID]; ok {
			cat = string(e.Category)
		}
		ackAt := ""
		if !r.AckReceivedAt.IsZero() {
			ackAt = r.AckReceivedAt.Format("2006-01-02")
		}
		closedAt := ""
		if !r.ClosedAt.IsZero() {
			closedAt = r.ClosedAt.Format("2006-01-02")
		}
		fmt.Printf("%s,%s,%s,%s,%s,%d,%s,%s,%s,%s\n",
			r.RequestID, r.NBFCID, r.NBFCName, cat, r.LifecycleState,
			r.EscalationLevel, r.CreatedAt.Format("2006-01-02"),
			ackAt, closedAt, r.Outcome)
	}
}

func sortedKeys(m map[string]int) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] })
	return keys
}
