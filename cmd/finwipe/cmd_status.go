package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/history"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show deletion request summary (alias: finwipe track --all)",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	dbPath := dbPath()
	hist, err := history.New(dbPath)
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	sum, err := hist.Summary()
	if err != nil {
		return fmt.Errorf("summary: %w", err)
	}

	fmt.Printf("\n📊 FinWipe Status\n")
	fmt.Printf("────────────────────────────\n")
	fmt.Printf("  Total active:   %d\n", sum["total"])
	fmt.Printf("  🆕 Initiated:  %d\n", sum["INITIATED"])
	fmt.Printf("  📨 Dispatched: %d\n", sum["DISPATCHED"])
	fmt.Printf("  ✅ Acked:      %d\n", sum["ACK_RECEIVED"])
	fmt.Printf("  🔺 Escalated:  %d\n", sum["escalated"])
	fmt.Printf("  ✔️  Closed:     %d\n", sum["CLOSED"])
	fmt.Println()

	if sum["DISPATCHED"] > 0 {
		fmt.Printf("⚠️  %d request(s) awaiting acknowledgment.\n", sum["DISPATCHED"])
		fmt.Println("  Run: finwipe track --all   to see details")
		fmt.Println("  Run: finwipe cron          to send follow-ups")
	}

	if sum["escalated"] > 0 {
		fmt.Printf("\n🔺 %d request(s) escalated.\n", sum["escalated"])
		fmt.Println("  Run: finwipe track --escalated   to see details")
	}

	fmt.Println()
	return nil
}
