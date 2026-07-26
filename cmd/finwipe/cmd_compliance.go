package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

// NBFCStats holds aggregate compliance metrics for one NBFC
type NBFCStats struct {
	ID            string
	Name          string
	Total         int
	Acknowledged  int
	Deleted       int
	Ignored       int
	Escalated     int
	AvgAckDays    float64
	AvgDeleteDays float64
	AckRate       float64
	DeleteRate    float64
	IgnoredRate   float64
}

var complianceCmd = &cobra.Command{
	Use:   "compliance",
	Short: "Track and report NBFC compliance rates (anonymized community data)",
	Long: `Track how NBFCs respond to deletion requests.

This command aggregates anonymized data from your FinWipe installation
and (optionally) compares against community averages to identify which
NBFCs take DPDPA seriously and which ignore requests.

Community data is contributed voluntarily and anonymized — no personal
request data is ever shared. Only aggregate compliance metrics.

Examples:
  finwipe compliance                          # Your compliance dashboard
  finwipe compliance --submit                 # Submit anonymized stats to community
  finwipe compliance --nbgf bajaj-finserv     # Per-NBFC detail
  finwipe compliance --export csv            # Export all data as CSV
  finwipe compliance --shame                 # List NBFCs with worst response rates`,
	RunE:  runCompliance,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var (
	complianceNBFC   string
	complianceExport  string
	complianceSubmit  bool
	complianceShame  bool
)

func init() {
	complianceCmd.Flags().StringVar(&complianceNBFC, "nbgf", "",
		"Show compliance for specific NBFC")
	complianceCmd.Flags().StringVar(&complianceExport, "export", "",
		"Export as format: csv, json")
	complianceCmd.Flags().BoolVar(&complianceSubmit, "submit", false,
		"Submit anonymized compliance stats to community database")
	complianceCmd.Flags().BoolVar(&complianceShame, "shame", false,
		"Show NBFCs with worst compliance rates")
	rootCmd.AddCommand(complianceCmd)
}

func runCompliance(cmd *cobra.Command, args []string) error {
	hist, err := history.New(dbPath())
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	all, err := hist.GetAll()
	if err != nil {
		return fmt.Errorf("get all: %w", err)
	}

	nbcfStats := make(map[string]*NBFCStats)
	overall := &NBFCStats{}

	for _, r := range all {
		stats := nbcfStats[r.NBFCID]
		if stats == nil {
			entities, _ := nbfc.Load(nbfcRegistryPath())
			found := false
			for _, e := range entities {
				if e.ID == r.NBFCID {
					stats = &NBFCStats{ID: r.NBFCID, Name: e.Name}
					nbcfStats[r.NBFCID] = stats
					found = true
					break
				}
			}
			if !found {
				stats = &NBFCStats{ID: r.NBFCID, Name: r.NBFCName}
				nbcfStats[r.NBFCID] = stats
			}
		}

		stats.Total++
		overall.Total++

		switch r.LifecycleState {
		case history.StateAckReceived, history.StateResponseOK:
			stats.Acknowledged++
			if stats.Total > 0 {
				stats.AckRate = float64(stats.Acknowledged) / float64(stats.Total) * 100
			}
			overall.Acknowledged++
			if overall.Total > 0 {
				overall.AckRate = float64(overall.Acknowledged) / float64(overall.Total) * 100
			}

		case history.StateClosed:
			stats.Deleted++
			if stats.Total > 0 {
				stats.DeleteRate = float64(stats.Deleted) / float64(stats.Total) * 100
			}
			overall.Deleted++
			if overall.Total > 0 {
				overall.DeleteRate = float64(overall.Deleted) / float64(overall.Total) * 100
			}

		case history.StateEscalated:
			stats.Escalated++
			overall.Escalated++

		default:
			if !r.AckDeadlineAt.IsZero() && time.Now().After(r.AckDeadlineAt) {
				stats.Ignored++
				if stats.Total > 0 {
					stats.IgnoredRate = float64(stats.Ignored) / float64(stats.Total) * 100
				}
				overall.Ignored++
				if overall.Total > 0 {
					overall.IgnoredRate = float64(overall.Ignored) / float64(overall.Total) * 100
				}
			}
		}

		if !r.DispatchedAt.IsZero() {
			if !r.AckReceivedAt.IsZero() {
				ackDays := r.AckReceivedAt.Sub(r.DispatchedAt).Hours() / 24
				if stats.Acknowledged > 1 {
					stats.AvgAckDays = (stats.AvgAckDays*float64(stats.Acknowledged-1) + ackDays) / float64(stats.Acknowledged)
				} else {
					stats.AvgAckDays = ackDays
				}
			}
			if !r.ClosedAt.IsZero() {
				deleteDays := r.ClosedAt.Sub(r.DispatchedAt).Hours() / 24
				if stats.Deleted > 1 {
					stats.AvgDeleteDays = (stats.AvgDeleteDays*float64(stats.Deleted-1) + deleteDays) / float64(stats.Deleted)
				} else {
					stats.AvgDeleteDays = deleteDays
				}
			}
		}
	}

	if complianceExport != "" {
		return exportCompliance(complianceExport, nbcfStats, overall)
	}

	if complianceNBFC != "" {
		if s, ok := nbcfStats[complianceNBFC]; ok {
			printNBFCStats(s)
			return nil
		}
		return fmt.Errorf("NBFC not found: %s", complianceNBFC)
	}

	if complianceShame {
		printShameList(nbcfStats)
		return nil
	}

	printComplianceDashboard(nbcfStats, overall)

	if complianceSubmit {
		fmt.Println("\n  Submission not yet implemented.")
		fmt.Println("  Community database coming soon.")
	}

	return nil
}

func printComplianceDashboard(stats map[string]*NBFCStats, overall *NBFCStats) {
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║         FinWipe — NBFC Compliance Dashboard                ║")
	fmt.Println("  ╠══════════════════════════════════════════════════════════════════╣")
	fmt.Println()
	fmt.Printf("  📊 Your Request History\n")
	fmt.Printf("  ──────────────────────────────────────────────────────────\n")
	fmt.Printf("  Total Requests:     %d\n", overall.Total)
	fmt.Printf("  Acknowledged:       %d (%.0f%%)\n", overall.Acknowledged, overall.AckRate)
	fmt.Printf("  Deleted:           %d (%.0f%%)\n", overall.Deleted, overall.DeleteRate)
	fmt.Printf("  Escalated:         %d\n", overall.Escalated)
	fmt.Printf("  Overdue/Unacked:  %d (%.0f%%)\n", overall.Ignored, overall.IgnoredRate)
	fmt.Println()

	if overall.AvgDeleteDays > 0 {
		fmt.Printf("  ⏱  Avg Time to Acknowledge:  %.1f days\n", overall.AvgAckDays)
		fmt.Printf("  ⏱  Avg Time to Delete:       %.1f days\n", overall.AvgDeleteDays)
		fmt.Println()
	}

	fmt.Printf("  📋 By NBFC (%d entities)\n", len(stats))
	fmt.Printf("  ──────────────────────────────────────────────────────────\n")

	var sorted []*NBFCStats
	for _, s := range stats {
		sorted = append(sorted, s)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].AckRate < sorted[j].AckRate
	})

	for i, s := range sorted {
		if i >= 15 {
			fmt.Printf("  ... and %d more\n", len(sorted)-15)
			break
		}
		ack := "⚠️"
		if s.AckRate >= 80 {
			ack = "✅"
		} else if s.AckRate >= 50 {
			ack = "🟡"
		}
		delete := "⚠️"
		if s.DeleteRate >= 70 {
			delete = "✅"
		} else if s.DeleteRate >= 40 {
			delete = "🟡"
		}
		fmt.Printf("  %2d. %-25s %s Ack:%.0f%% %s Del:%.0f%%\n",
			i+1, truncate(s.Name, 25), ack, s.AckRate, delete, s.DeleteRate)
	}

	fmt.Println()
	fmt.Println("  ═══════════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("  Legend: ✅ Good (80%+) | 🟡 Fair (50-79%) | ⚠️ Poor (<50%)")
	fmt.Println()
	fmt.Println("  Commands:")
	fmt.Println("    finwipe compliance --shame        # Worst offenders")
	fmt.Println("    finwipe compliance --nbgf <id>    # Per-NBFC detail")
	fmt.Println("    finwipe compliance --export csv   # Export all data")
}

func printNBFCStats(stats *NBFCStats) {
	fmt.Println()
	fmt.Printf("  📋 %s — Compliance Report\n", stats.Name)
	fmt.Printf("  ──────────────────────────────────────────────────────────\n")
	fmt.Printf("  Total Requests:    %d\n", stats.Total)
	fmt.Printf("  Acknowledged:      %d (%.0f%%)\n", stats.Acknowledged, stats.AckRate)
	fmt.Printf("  Deleted:          %d (%.0f%%)\n", stats.Deleted, stats.DeleteRate)
	fmt.Printf("  Escalated:        %d\n", stats.Escalated)
	fmt.Printf("  Ignored/Overdue:  %d (%.0f%%)\n", stats.Ignored, stats.IgnoredRate)
	fmt.Println()
	if stats.AvgAckDays > 0 {
		fmt.Printf("  Avg Acknowledge Time:   %.1f days\n", stats.AvgAckDays)
	}
	if stats.AvgDeleteDays > 0 {
		fmt.Printf("  Avg Deletion Time:     %.1f days\n", stats.AvgDeleteDays)
	}
	fmt.Println()

	verdict := "🟡 NEEDS IMPROVEMENT"
	if stats.AckRate >= 90 && stats.DeleteRate >= 80 {
		verdict = "✅ COMPLIANT"
	} else if stats.AckRate < 50 || stats.DeleteRate < 30 {
		verdict = "🚨 NON-COMPLIANT — Consider escalation"
	}
	fmt.Printf("  Verdict: %s\n", verdict)
	fmt.Println()
}

func printShameList(stats map[string]*NBFCStats) {
	var shame []*NBFCStats
	for _, s := range stats {
		if s.Total >= 2 {
			shame = append(shame, s)
		}
	}
	sort.Slice(shame, func(i, j int) bool {
		return shame[i].AckRate < shame[j].AckRate
	})

	fmt.Println()
	fmt.Println("  🚨 NBFC SHAME LIST — Worst DPDPA Compliance")
	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	fmt.Println("  NBFCs with lowest acknowledgment rates (2+ requests tracked)")
	fmt.Println()

	for i, s := range shame {
		if i >= 20 {
			fmt.Printf("  ... and %d more\n", len(shame)-20)
			break
		}
		icon := "✅"
		if s.AckRate < 30 {
			icon = "🚨"
		} else if s.AckRate < 60 {
			icon = "🟡"
		}
		fmt.Printf("  %2d. %s %-28s %.0f%% acknowledged\n", i+1, icon, truncate(s.Name, 28), s.AckRate)
	}

	fmt.Println()
	fmt.Println("  🚨 = 0-29% | 🟡 = 30-59% | ✅ = 60%+ acknowledgment")
	fmt.Println()
	fmt.Println("  These NBFCs are ignoring DPDPA deletion requests.")
	fmt.Println("  Escalation options:")
	fmt.Println("    finwipe escalate --request-id <ID> --to dpd_board")
	fmt.Println("    finwipe escalate --request-id <ID> --to rbi_ombudsman")
}

func exportCompliance(format string, stats map[string]*NBFCStats, overall *NBFCStats) error {
	switch format {
	case "csv":
		path := filepath.Join(os.Getenv("HOME"), ".finwipe", "compliance_export.csv")
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		w := csv.NewWriter(f)
		w.Write([]string{"NBFC ID", "NBFC Name", "Total", "Acknowledged", "Deleted", "Escalated", "Ignored", "Ack Rate %", "Delete Rate %"})
		for _, s := range stats {
			w.Write([]string{
				s.ID, s.Name,
				fmt.Sprintf("%d", s.Total),
				fmt.Sprintf("%d", s.Acknowledged),
				fmt.Sprintf("%d", s.Deleted),
				fmt.Sprintf("%d", s.Escalated),
				fmt.Sprintf("%d", s.Ignored),
				fmt.Sprintf("%.0f", s.AckRate),
				fmt.Sprintf("%.0f", s.DeleteRate),
			})
		}
		w.Flush()
		fmt.Printf("\n  📄 Exported to: %s\n", path)

	case "json":
		fmt.Println("\n  {")
		fmt.Printf("    \"total_requests\": %d,\n", overall.Total)
		fmt.Printf("    \"acknowledged\": %d,\n", overall.Acknowledged)
		fmt.Printf("    \"deleted\": %d,\n", overall.Deleted)
		fmt.Printf("    \"nbcfs_tracked\": %d\n", len(stats))
		fmt.Println("  }")
	}
	return nil
}
