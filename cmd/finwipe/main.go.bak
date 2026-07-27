package main

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile       string
	dryRun        bool
	ackRequestID  string // used by ack.go, escalate.go, close.go
	escRequestID  string
	closeRequestID string
)

var rootCmd = &cobra.Command{
	Use:   "finwipe",
	Short: "FinWipe — DIY NBFC data deletion tracker for India",
	Long: `FinWipe tracks your DPDPA 2023 data deletion requests through their
full lifecycle: send → acknowledge → follow-up → escalate → close.

Every request gets a unique DPR-ID (DPR-2026-000001) for full auditability.

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 GETTING STARTED (5 steps)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  1. finwipe init                    → Set up your profile (name, email, phone, SMTP)
  2. finwipe update-registry         → Download the latest NBFC list (run once)
  3. finwipe list --search bajaj    → Find an NBFC in the registry
  4. finwipe new --nbfc bajaj-finserv → Create a deletion request (get DPR-ID)
  5. finwipe send --dry-run          → Preview the email before sending
  6. finwipe send                    → Send the deletion request

After the NBFC responds:
  finwipe ack --request-id DPR-2026-000001  → Record their acknowledgment
  finwipe close --request-id DPR-2026-000001  → Close the request with outcome

Need help?  finwipe wizard  → Interactive step-by-step guide
Full docs: https://github.com/das-rebel/finwipe
`,
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

	// Interactive + Evidence
	rootCmd.AddCommand(evidenceCmd)   // Evidence management

	// Utility
	rootCmd.AddCommand(letterCmd)
	rootCmd.AddCommand(cicCmd)
	rootCmd.AddCommand(parseCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// dbPath returns the SQLite history database path
func dbPath() string {
	home, _ := os.UserHomeDir()
	return home + "/.finwipe/history.db"
}
