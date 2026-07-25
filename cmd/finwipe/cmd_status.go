package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/history"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show deletion request history and status",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	histDBPath := filepath.Join(home, ".finwipe", "history.db")

	hist, err := history.New(histDBPath)
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	total, pending, sent, ack, comp, fail, manual := hist.Summary()

	fmt.Printf("\n📊 FinWipe Status\n")
	fmt.Printf("────────────────────────────\n")
	fmt.Printf("  Total requests: %d\n", total)
	fmt.Printf("  ⏳ Pending:     %d\n", pending)
	fmt.Printf("  ✉️  Sent:        %d\n", sent)
	fmt.Printf("  ✅ Acknowledged: %d\n", ack)
	fmt.Printf("  ✔️  Completed:   %d\n", comp)
	fmt.Printf("  ❌ Failed:      %d\n", fail)
	fmt.Printf("  🏛️  Manual req:  %d\n", manual)
	fmt.Println()

	// Show recent failed
	if fail > 0 {
		failed, _ := hist.GetByStatus("failed")
		fmt.Println("❌ Failed requests:")
		for _, r := range failed {
			fmt.Printf("  • %s [%s]: %s\n", r.NBFCName, r.Channel, r.FailureReason)
		}
		fmt.Println()
	}

	// Show manual required
	if manual > 0 {
		man, _ := hist.GetByStatus("manual_required")
		fmt.Println("🏛️  Requires manual action:")
		for _, r := range man {
			fmt.Printf("  • %s [%s]\n    %s\n", r.NBFCName, r.Channel, r.Notes)
		}
		fmt.Println()
	}

	// Show pending
	if pending > 0 {
		pend, _ := hist.GetByStatus("pending")
		fmt.Printf("⏳ Pending (%d):\n", len(pend))
		for _, r := range pend {
			if r.SentAt.IsZero() {
				fmt.Printf("  • %s [%s]: not yet sent\n", r.NBFCName, r.Channel)
			} else {
				fmt.Printf("  • %s [%s]: sent %s\n", r.NBFCName, r.Channel, r.SentAt.Format("02 Jan 2006"))
			}
		}
		fmt.Println()
	}

	// Show acknowledged
	if ack > 0 {
		acks, _ := hist.GetByStatus("acknowledged")
		fmt.Printf("✅ Acknowledged (%d):\n", len(acks))
		for _, r := range acks {
			days := int(time.Since(r.AcknowledgedAt).Hours() / 24)
			ref := ""
			if r.ExternalRef != "" {
				ref = " | Ref: " + r.ExternalRef
			}
			fmt.Printf("  • %s [%s]: acknowledged %d days ago%s\n",
				r.NBFCName, r.Channel, days, ref)
		}
		fmt.Println()
	}

	return nil
}
