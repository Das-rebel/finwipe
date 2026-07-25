package main

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	dryRun bool
)

var rootCmd = &cobra.Command{
	Use:   "finwipe",
	Short: "FinWipe — DIY RBI data rights toolkit for India",
	Long: `FinWipe sends data deletion requests to NBFCs, fintechs, and data brokers
that hold your financial data — under India's DPDPA 2023 and RBI Digital Lending Guidelines.

No data ever leaves your server. All emails sent from YOUR email.`,
}

func main() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.finwipe/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "preview what would be sent, don't actually send")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(sendCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(letterCmd)
	rootCmd.AddCommand(cicCmd)
	rootCmd.AddCommand(parseCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
