package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	cfgFile       string
	dryRun        bool
	ackRequestID  string
	escRequestID  string
	closeRequestID string
)

var version = "0.1.7"

var rootCmd = &cobra.Command{
	Use:   "finwipe",
	Short: "FinWipe – DIY NBFC data deletion tracker for India",
	Long: `FinWipe tracks your DPDPA 2023 data deletion requests through their full lifecycle:
  send → acknowledge → follow-up → escalate → close.

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
Full docs: https://github.com/das-rebel/finwipe`,
}

func main() {
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "--version" || args[0] == "-v") {
		fmt.Printf("finwipe %s\n", version)
		return
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default: ~/.finwipe/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false,
		"preview what would happen without making changes")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(sendCmd)
	rootCmd.AddCommand(trackCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(letterCmd)
	rootCmd.AddCommand(forwardCmd)
	rootCmd.AddCommand(checkInboxCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(cloudCmd)
	rootCmd.AddCommand(wizardCmd)
	rootCmd.AddCommand(evidenceCmd)
	rootCmd.AddCommand(ackCmd)
	rootCmd.AddCommand(closeCmd)
	rootCmd.AddCommand(cronCmd)
	rootCmd.AddCommand(massCmd)
	rootCmd.AddCommand(complianceCmd)
	rootCmd.AddCommand(updateRegistryCmd)
	rootCmd.AddCommand(scrapeCmd)
	rootCmd.AddCommand(discoverCmd)
	rootCmd.AddCommand(aaDiscoverCmd)
	rootCmd.AddCommand(bankStatementCmd)
	rootCmd.AddCommand(bureauCmd)
	rootCmd.AddCommand(discoverCibilCmd)
	rootCmd.AddCommand(emailDiscoveryCmd)
	rootCmd.AddCommand(whatsappDiscoverCmd)
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(askCmd)
	rootCmd.AddCommand(portabilityCmd)
	rootCmd.AddCommand(parseCmd)
	rootCmd.AddCommand(cicCmd)
}

func dbPath() string {
	dbPath, _ := os.UserHomeDir()
	return filepath.Join(dbPath, ".finwipe", "finwipe.db")
}

func nbfcRegistryPath() string {
	// Return empty string — Load() uses embedded data as fallback.
	// Commands can pass an explicit path to override.
	return ""
}

func dataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".finwipe")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
