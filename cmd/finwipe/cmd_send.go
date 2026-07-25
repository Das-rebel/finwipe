package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/email"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send deletion emails to all configured NBFCs",
	RunE:  runSend,
}

var (
	excludeCategories string // comma-separated
	includeIDs       string // comma-separated
	rateLimitMs      int
)

func init() {
	sendCmd.Flags().StringVar(&excludeCategories, "exclude-category", "", "Exclude NBFCs by category (e.g., bank,hfc)")
	sendCmd.Flags().StringVar(&includeIDs, "include", "", "Only send to these NBFC IDs (comma-separated)")
	sendCmd.Flags().IntVar(&rateLimitMs, "rate-limit", 1000, "Milliseconds between emails (Gmail: 1000+ recommended)")
}

func runSend(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Load NBFCs
	exePath, _ := os.Executable()
	nbfcPath := filepath.Join(filepath.Dir(exePath), "data", "nbfcs.yaml")
	if _, err := os.Stat(nbfcPath); err != nil {
		nbfcPath = "./data/nbfcs.yaml"
	}

	allNBFCs, err := nbfc.Load(nbfcPath)
	if err != nil {
		return fmt.Errorf("load NBFCs: %w", err)
	}

	// Filter
	var targetNBFCs []nbfc.Entity
	if includeIDs != "" {
		includeMap := make(map[string]bool)
		for _, id := range splitCSV(includeIDs) {
			includeMap[id] = true
		}
		for _, n := range allNBFCs {
			if includeMap[n.ID] {
				targetNBFCs = append(targetNBFCs, n)
			}
		}
	} else {
		excludeMap := make(map[nbfc.Category]bool)
		for _, cat := range splitCSV(excludeCategories) {
			excludeMap[nbfc.Category(cat)] = true
		}
		for _, n := range allNBFCs {
			if !excludeMap[n.Category] {
				targetNBFCs = append(targetNBFCs, n)
			}
		}
	}

	if len(targetNBFCs) == 0 {
		fmt.Println("No NBFCs to send to (check --include or --exclude-category)")
		return nil
	}

	// Init history DB
	home, _ := os.UserHomeDir()
	histDBPath := filepath.Join(home, ".finwipe", "history.db")
	hist, err := history.New(histDBPath)
	if err != nil {
		return fmt.Errorf("history db: %w", err)
	}
	defer hist.Close()

	// Record all requests
	for _, n := range targetNBFCs {
		hist.RecordRequest(n.ID, n.Name, "email", "pending")
	}

	// Dry run
	if dryRun {
		fmt.Printf("\n🔍 DRY RUN — No emails will be sent\n\n")
		fmt.Printf("Would send to %d NBFCs:\n\n", len(targetNBFCs))
		for _, n := range targetNBFCs {
			fmt.Printf("  ✉️  %-30s <%s>\n", n.Name, n.GrievanceEmail)
		}
		fmt.Printf("\nConfigure SMTP: %s\n", config.DefaultPath())
		return nil
	}

	// Send
	fmt.Printf("\n🗑️  Sending deletion emails to %d NBFCs...\n\n", len(targetNBFCs))

	if cfg.SMTP.Password == "" {
		return fmt.Errorf("SMTP not configured. Run: finwipe init")
	}
	sender := email.New(&cfg.SMTP)
	sent, failed := sender.SendBatch(targetNBFCs, cfg.Profile, "", rateLimitMs)

	// Update history
	for _, n := range targetNBFCs {
		hist.MarkSent(n.ID, "email")
	}
	for _, f := range failed {
		hist.UpdateStatus("unknown", "email", "failed", f)
	}

	fmt.Printf("\n✅ Sent: %d | Failed: %d\n", sent, len(failed))
	if len(failed) > 0 {
		fmt.Println("\nFailed:")
		for _, f := range failed {
			fmt.Printf("  ❌ %s\n", f)
		}
	}

	// Summary
	total, pending, psent, ack, comp, fail, manual := hist.Summary()
	fmt.Printf("\n📊 History Summary:\n")
	fmt.Printf("  Total: %d | Pending: %d | Sent: %d | Ack: %d | Done: %d | Failed: %d | Manual: %d\n",
		total, pending, psent, ack, comp, fail, manual)

	return nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, part := range splitString(s, ",") {
		trimmed := trimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitString(s, sep string) []string {
	var r []string
	for i := 0; i < len(s); {
		j := indexOf(s, sep, i)
		if j < 0 {
			r = append(r, s[i:])
			break
		}
		r = append(r, s[i:j])
		i = j + len(sep)
	}
	return r
}

func indexOf(s, substr string, start int) int {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\n' || s[j-1] == '\r') {
		j--
	}
	return s[i:j]
}
