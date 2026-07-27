package main

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile       string
	dryRun        bool
	ackRequestID  string // used by ack.go, escalate.go, close.go, followup.go
	escRequestID  string
	closeRequestID string
)

var rootCmd = &cobra.Command{
	Use:   "finwipe",
	Short: "FinWipe — DIY NBFC data deletion tracker for India",
	Long: `FinWipe tracks your DPDPA 2023 data deletion requests through their
full lifecycle: send → acknowledge → follow-up → escalate → close.

Every request gets a unique DPR-ID (DPR-2026-000001) for full auditability.`,
}

func main() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default: ~/.finwipe/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false,
		"preview what would happen without making changes")

	// Core setup commands
	rootCmd.AddCommand(initCmd)

	// NBFC registry
	rootCmd.AddCommand(listCmd)

	// Request lifecycle commands
	rootCmd.AddCommand(newCmd)     // Create new request
	rootCmd.AddCommand(sendCmd)     // Dispatch request
	rootCmd.AddCommand(trackCmd)    // Track request lifecycle + audit trail
	rootCmd.AddCommand(ackCmd)      // Record NBFC acknowledgment
	rootCmd.AddCommand(escalateCmd) // Escalate to RBI/DPDP/Consumer Forum
	rootCmd.AddCommand(closeCmd)    // Close with outcome

	// Automation
	rootCmd.AddCommand(cronCmd) // Daily follow-up + escalation automation

	// Dashboard
	rootCmd.AddCommand(reportCmd) // Compliance dashboard and reporting

	// Email forwarding discovery (CRED/Fold model)
	rootCmd.AddCommand(forwardCmd)
	rootCmd.AddCommand(checkInboxCmd)
	rootCmd.AddCommand(syncCmd)       // Cloud sync + auto-create
	rootCmd.AddCommand(cloudCmd)      // Cloud status check

	// Interactive + Evidence
	rootCmd.AddCommand(evidenceCmd)   // Evidence management

	// Utility
	rootCmd.AddCommand(letterCmd)
	rootCmd.AddCommand(cicCmd)
	rootCmd.AddCommand(parseCmd)
	rootCmd.AddCommand(scrapeCmd)           // Scrape privacy policy for grievance email
	rootCmd.AddCommand(updateRegistryCmd)    // Update registry from awesome-fintech-india

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// dbPath returns the SQLite history database path
func dbPath() string {
	home, _ := os.UserHomeDir()
	return home + "/.finwipe/history.db"
}
